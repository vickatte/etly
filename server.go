package etly

import (
	"fmt"
	"log"
	"net/http"

	"context"
	"github.com/viant/toolbox"
)

const uriBasePath = "/etly/"

type Server struct {
	serviceRouter *toolbox.ServiceRouter
	config        *ServerConfig
	Service       *Service
}

func (s *Server) Start() (err error) {
	err = s.Service.Start()
	if err != nil {
		return err
	}
	defer s.Service.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc(uriBasePath, func(writer http.ResponseWriter, reader *http.Request) {
		err := s.serviceRouter.Route(writer, reader)
		if err != nil {
			http.Error(writer, fmt.Sprintf("%v", err), http.StatusInternalServerError)
			return
		}
	})
	logger.Printf("Starting ETL service on port %v", s.config.Port)
	//Start start server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.config.Port), mux))
	return nil
}

//Shutdown server (Provided explicitly for graceful shutdown)
func (s *Server) Stop() (err error) {
	//Stopping the transferring service
	return s.Stop()
}
func NewServer(config *ServerConfig, transferConfig *TransferConfig) (*Server, error) {
	return NewServerWithContext(context.TODO(), config, transferConfig)
}
func NewServerWithContext(context context.Context, config *ServerConfig, transferConfig *TransferConfig) (*Server, error) {
	service, err := NewServiceWithContext(context, config, transferConfig)
	if err != nil {
		return nil, err
	}
	router := toolbox.NewServiceRouter(
		toolbox.ServiceRouting{
			HTTPMethod: "GET",
			URI:        uriBasePath + "tasks/{ids}",
			Handler:    service.GetTasks,
			Parameters: []string{"@httpRequest", "ids"},
		},
		toolbox.ServiceRouting{
			HTTPMethod: "GET",
			URI:        uriBasePath + "tasklist/",
			Handler:    service.GetTasksList,
			Parameters: []string{"@httpRequest"},
		},
		toolbox.ServiceRouting{
			HTTPMethod: "GET",
			URI:        uriBasePath + "tasks",
			Handler:    service.GetTasksByStatus,
			Parameters: []string{"status"},
		},
		toolbox.ServiceRouting{
			HTTPMethod: "GET",
			URI:        uriBasePath + "status",
			Handler:    service.Status,
			Parameters: []string{},
		},
		toolbox.ServiceRouting{
			HTTPMethod: "GET",
			URI:        uriBasePath + "errors",
			Handler:    service.GetErrors,
			Parameters: []string{},
		},
		toolbox.ServiceRouting{
			HTTPMethod: "GET",
			URI:        uriBasePath + "info/{name}",
			Handler:    service.ProcessingStatus,
			Parameters: []string{"name"},
		},
		toolbox.ServiceRouting{
			HTTPMethod: "POST",
			URI:        uriBasePath + "transfer",
			Handler:    service.transferObjectService.Transfer,
			Parameters: []string{"request"},
		},

		toolbox.ServiceRouting{
			HTTPMethod: "POST",
			URI:        uriBasePath + "transferOnce",
			Handler:    service.TransferOnce,
			Parameters: []string{"request"},
		},
		toolbox.ServiceRouting{
			HTTPMethod: "GET",
			URI:        uriBasePath + "version",
			Handler:    service.Version,
			Parameters: []string{},
		},
	)
	result := &Server{
		serviceRouter: router,
		config:        config,
		Service:       service,
	}
	return result, nil
}
