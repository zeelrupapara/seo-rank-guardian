package v1

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/authz"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"github.com/zeelrupapara/seo-rank-guardian/utils"
)

// --- Request/Response types ---

type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Role     string `json:"role" validate:"required,min=1,max=50"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" validate:"required,min=1,max=50"`
}

type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

type AdminStatsData struct {
	TotalUsers  int64 `json:"total_users"`
	ActiveUsers int64 `json:"active_users"`
	AdminUsers  int64 `json:"admin_users"`
	TotalJobs   int64 `json:"total_jobs"`
	ActiveJobs  int64 `json:"active_jobs"`
	TotalRuns   int64 `json:"total_runs"`
	RunsToday   int64 `json:"runs_today"`

	RecentUsers []AdminRecentUser `json:"recent_users"`
	RecentRuns  []AdminRecentRun  `json:"recent_runs"`
	UserGrowth  []AdminGrowthPoint `json:"user_growth"`
	RunActivity []AdminGrowthPoint `json:"run_activity"`
}

type AdminRecentUser struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"created_at"`
}

type AdminRecentRun struct {
	ID        uint   `json:"id"`
	JobID     uint   `json:"job_id"`
	JobName   string `json:"job_name"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

type AdminGrowthPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type AdminUserData struct {
	model.User
	JobCount int64 `json:"job_count"`
}

type AdminJobListItem struct {
	model.Job
	OwnerUsername string `json:"owner_username"`
	OwnerEmail   string `json:"owner_email"`
	KeywordCount int    `json:"keyword_count"`
	RegionCount  int    `json:"region_count"`
}

// --- Handlers ---

// AdminGetStats returns system-wide statistics.
func (h *HttpServer) AdminGetStats(c *fiber.Ctx) error {
	var stats AdminStatsData

	h.DB.Model(&model.User{}).Count(&stats.TotalUsers)
	h.DB.Model(&model.User{}).Where("is_active = ?", true).Count(&stats.ActiveUsers)
	h.DB.Model(&model.User{}).Where("role = ?", "admin").Count(&stats.AdminUsers)
	h.DB.Model(&model.Job{}).Count(&stats.TotalJobs)
	h.DB.Model(&model.Job{}).Where("is_active = ?", true).Count(&stats.ActiveJobs)
	h.DB.Model(&model.JobRun{}).Count(&stats.TotalRuns)

	todayStart := time.Now().Truncate(24 * time.Hour).UnixNano()
	h.DB.Model(&model.JobRun{}).Where("created_at >= ?", todayStart).Count(&stats.RunsToday)

	// Recent 5 users
	var recentUsers []model.User
	h.DB.Order("created_at DESC").Limit(5).Find(&recentUsers)
	stats.RecentUsers = make([]AdminRecentUser, 0, len(recentUsers))
	for _, u := range recentUsers {
		stats.RecentUsers = append(stats.RecentUsers, AdminRecentUser{
			ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role, CreatedAt: u.CreatedAt,
		})
	}

	// Recent 5 runs with job name
	type runRow struct {
		ID        uint
		JobID     uint
		JobName   string
		Status    string
		CreatedAt int64
	}
	var recentRuns []runRow
	h.DB.Table("srg_job_runs").
		Select("srg_job_runs.id, srg_job_runs.job_id, srg_jobs.name as job_name, srg_job_runs.status, srg_job_runs.created_at").
		Joins("LEFT JOIN srg_jobs ON srg_jobs.id = srg_job_runs.job_id").
		Order("srg_job_runs.created_at DESC").Limit(5).
		Scan(&recentRuns)
	stats.RecentRuns = make([]AdminRecentRun, 0, len(recentRuns))
	for _, r := range recentRuns {
		stats.RecentRuns = append(stats.RecentRuns, AdminRecentRun{
			ID: r.ID, JobID: r.JobID, JobName: r.JobName, Status: r.Status, CreatedAt: r.CreatedAt,
		})
	}

	// User growth — last 7 days
	stats.UserGrowth = h.growthLast7Days("srg_users")

	// Run activity — last 7 days
	stats.RunActivity = h.growthLast7Days("srg_job_runs")

	return httputil.SuccessResponse(c, fiber.StatusOK, stats, "Admin stats retrieved")
}

// growthLast7Days returns daily counts for the last 7 days from a table with created_at (nanoseconds).
func (h *HttpServer) growthLast7Days(table string) []AdminGrowthPoint {
	points := make([]AdminGrowthPoint, 7)
	now := time.Now()
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayStart := day.Truncate(24 * time.Hour).UnixNano()
		dayEnd := day.Truncate(24*time.Hour).Add(24*time.Hour).UnixNano() - 1
		var count int64
		h.DB.Table(table).Where("created_at >= ? AND created_at <= ? AND deleted_at = 0", dayStart, dayEnd).Count(&count)
		points[6-i] = AdminGrowthPoint{
			Date:  day.Format("Jan 02"),
			Count: count,
		}
	}
	return points
}

// AdminListUsers returns a paginated list of all users.
func (h *HttpServer) AdminListUsers(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	query := h.DB.Model(&model.User{})

	if search := c.Query("search"); search != "" {
		like := "%" + search + "%"
		query = query.Where("(username ILIKE ? OR email ILIKE ?)", like, like)
	}

	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}

	if status := c.Query("status"); status == "active" {
		query = query.Where("is_active = ?", true)
	} else if status == "inactive" {
		query = query.Where("is_active = ?", false)
	}

	var total int64
	query.Count(&total)

	var users []model.User
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list users")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": users,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Users retrieved")
}

// AdminGetUser returns a single user with job count.
func (h *HttpServer) AdminGetUser(c *fiber.Ctx) error {
	userId, err := strconv.ParseUint(c.Params("userId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid user ID")
	}

	var user model.User
	if err := h.DB.First(&user, userId).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	var jobCount int64
	h.DB.Model(&model.Job{}).Where("user_id = ?", userId).Count(&jobCount)

	return httputil.SuccessResponse(c, fiber.StatusOK, AdminUserData{
		User:     user,
		JobCount: jobCount,
	}, "User retrieved")
}

// AdminCreateUser creates a new user (the ONLY way to create admin accounts).
func (h *HttpServer) AdminCreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	var existing model.User
	if err := h.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrUserAlreadyExist.Error(), "An account with this email already exists")
	}
	if err := h.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrUserAlreadyExist.Error(), "This username is already taken")
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to hash password")
	}

	user := model.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      req.Role,
		IsActive:  true,
		AvatarURL: model.DefaultAvatarURL(req.Username),
	}

	if err := h.DB.Create(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create user")
	}

	return httputil.SuccessResponse(c, fiber.StatusCreated, user, "User created successfully")
}

// AdminUpdateUserRole changes a user's role. Self-protection: cannot change own role.
func (h *HttpServer) AdminUpdateUserRole(c *fiber.Ctx) error {
	currentUserID, _ := c.Locals("userId").(uint)

	targetID, err := strconv.ParseUint(c.Params("userId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid user ID")
	}

	if uint(targetID) == currentUserID {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Cannot change your own role")
	}

	var req UpdateUserRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid role")
	}

	// Verify role has at least one policy
	policies := h.Middleware.Authz.GetAllPolicies()
	roleExists := false
	for _, p := range policies {
		if len(p) > 0 && p[0] == req.Role {
			roleExists = true
			break
		}
	}
	if !roleExists {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Role '"+req.Role+"' has no policies. Create policies for this role first.")
	}

	var user model.User
	if err := h.DB.First(&user, targetID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	user.Role = req.Role
	if err := h.DB.Save(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update role")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, user, "User role updated")
}

// AdminUpdateUserStatus activates or deactivates a user. Self-protection: cannot deactivate yourself.
func (h *HttpServer) AdminUpdateUserStatus(c *fiber.Ctx) error {
	currentUserID, _ := c.Locals("userId").(uint)

	targetID, err := strconv.ParseUint(c.Params("userId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid user ID")
	}

	if uint(targetID) == currentUserID {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Cannot change your own status")
	}

	var req UpdateUserStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	var user model.User
	if err := h.DB.First(&user, targetID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	user.IsActive = req.IsActive
	if err := h.DB.Save(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update status")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, user, "User status updated")
}

// AdminDeleteUser soft-deletes a user. Self-protection: cannot delete yourself.
func (h *HttpServer) AdminDeleteUser(c *fiber.Ctx) error {
	currentUserID, _ := c.Locals("userId").(uint)

	targetID, err := strconv.ParseUint(c.Params("userId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid user ID")
	}

	if uint(targetID) == currentUserID {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Cannot delete your own account")
	}

	var user model.User
	if err := h.DB.First(&user, targetID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	if err := h.DB.Delete(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to delete user")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "User deleted successfully")
}

// AdminListJobs returns all jobs across all users with owner info.
func (h *HttpServer) AdminListJobs(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	query := h.DB.Model(&model.Job{})

	if search := c.Query("search"); search != "" {
		like := "%" + search + "%"
		query = query.Where("(name ILIKE ? OR domain ILIKE ?)", like, like)
	}

	if status := c.Query("status"); status == "active" {
		query = query.Where("is_active = ?", true)
	} else if status == "inactive" {
		query = query.Where("is_active = ?", false)
	}

	if userIdFilter := c.Query("user_id"); userIdFilter != "" {
		query = query.Where("user_id = ?", userIdFilter)
	}

	var total int64
	query.Count(&total)

	var jobs []model.Job
	if err := query.Preload("Keywords").Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&jobs).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list jobs")
	}

	// Collect unique user IDs to fetch owner info
	userIDs := make(map[uint]bool)
	for _, j := range jobs {
		userIDs[j.UserID] = true
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	var users []model.User
	if len(ids) > 0 {
		h.DB.Where("id IN ?", ids).Find(&users)
	}
	userMap := make(map[uint]model.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	items := make([]AdminJobListItem, 0, len(jobs))
	for _, j := range jobs {
		owner := userMap[j.UserID]
		items = append(items, AdminJobListItem{
			Job:          j,
			OwnerUsername: owner.Username,
			OwnerEmail:   owner.Email,
			KeywordCount: len(j.Keywords),
			RegionCount:  len(j.Regions),
		})
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": items,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Jobs retrieved")
}

// AdminGetJob returns any job by ID without ownership check.
func (h *HttpServer) AdminGetJob(c *fiber.Ctx) error {
	jobId, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	var job model.Job
	if err := h.DB.Preload("Keywords").First(&job, jobId).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	var totalRuns int64
	h.DB.Model(&model.JobRun{}).Where("job_id = ?", jobId).Count(&totalRuns)

	var lastRun model.JobRun
	h.DB.Where("job_id = ?", jobId).Order("created_at DESC").First(&lastRun)

	var owner model.User
	h.DB.First(&owner, job.UserID)

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"job":            job,
		"total_runs":     totalRuns,
		"last_run":       lastRun,
		"owner_username": owner.Username,
		"owner_email":    owner.Email,
	}, "Job retrieved")
}

// --- Policy Management ---

type PolicyRequest struct {
	Role     string `json:"role" validate:"required,min=1,max=50"`
	Resource string `json:"resource" validate:"required,min=1,max=50"`
	Action   string `json:"action" validate:"required,oneof=read write delete"`
}

type PolicyItem struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
}

// AdminListPolicies returns all Casbin policies grouped by role.
func (h *HttpServer) AdminListPolicies(c *fiber.Ctx) error {
	policies := h.Middleware.Authz.GetAllPolicies()

	// Group by role → resource → actions
	roleMap := make(map[string]map[string][]string)
	for _, p := range policies {
		if len(p) < 3 {
			continue
		}
		role, resource, action := p[0], p[1], p[2]
		if roleMap[role] == nil {
			roleMap[role] = make(map[string][]string)
		}
		roleMap[role][resource] = append(roleMap[role][resource], action)
	}

	// Convert to structured response
	result := make(map[string][]PolicyItem)
	for role, resources := range roleMap {
		items := make([]PolicyItem, 0, len(resources))
		for resource, actions := range resources {
			items = append(items, PolicyItem{Resource: resource, Actions: actions})
		}
		result[role] = items
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, result, "Policies retrieved")
}

// AdminAddPolicy adds a Casbin policy.
func (h *HttpServer) AdminAddPolicy(c *fiber.Ctx) error {
	var req PolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}
	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	// Validate resource is known
	validResource := false
	for _, r := range authz.AllResources() {
		if r == req.Resource {
			validResource = true
			break
		}
	}
	if !validResource {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Unknown resource: "+req.Resource)
	}

	added, err := h.Middleware.Authz.AddPolicy(req.Role, req.Resource, req.Action)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to add policy")
	}
	if !added {
		return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrConflict.Error(), "Policy already exists")
	}

	return httputil.SuccessResponse(c, fiber.StatusCreated, fiber.Map{
		"role": req.Role, "resource": req.Resource, "action": req.Action,
	}, "Policy added")
}

// AdminRemovePolicy removes a Casbin policy. Self-protection: cannot remove admin's policies access.
func (h *HttpServer) AdminRemovePolicy(c *fiber.Ctx) error {
	var req PolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}
	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	// Self-protection: cannot remove admin access to policies
	if req.Role == "admin" && req.Resource == authz.ResourcePolicies {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Cannot remove admin's policy management access")
	}

	removed, err := h.Middleware.Authz.RemovePolicy(req.Role, req.Resource, req.Action)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to remove policy")
	}
	if !removed {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrNotFound.Error(), "Policy not found")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "Policy removed")
}

// AdminListResources returns all available resources and actions.
func (h *HttpServer) AdminListResources(c *fiber.Ctx) error {
	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"resources": authz.AllResources(),
		"actions":   authz.AllActions(),
	}, "Resources retrieved")
}

// AdminListRoles returns distinct roles from policies and users.
func (h *HttpServer) AdminListRoles(c *fiber.Ctx) error {
	// Get roles from Casbin policies
	policies := h.Middleware.Authz.GetAllPolicies()
	roleSet := make(map[string]bool)
	for _, p := range policies {
		if len(p) > 0 {
			roleSet[p[0]] = true
		}
	}

	// Also get roles from users table
	var userRoles []string
	h.DB.Model(&model.User{}).Distinct("role").Pluck("role", &userRoles)
	for _, r := range userRoles {
		roleSet[r] = true
	}

	roles := make([]string, 0, len(roleSet))
	for r := range roleSet {
		roles = append(roles, r)
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, roles, "Roles retrieved")
}
