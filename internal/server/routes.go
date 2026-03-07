package server

func (s *Server) Register() {
	s.HttpServer.RegisterV1()
}
