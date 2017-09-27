package etly

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/viant/toolbox"
	"github.com/viant/toolbox/storage"
)

var logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

const MaxStatusTaskCount = 10

type Service struct {
	config                *ServerConfig
	transferConfig        *TransferConfig
	isRunning             int32
	stopNotification      chan bool
	transferService       *transferService
	transferObjectService TransferObjectService
	taskRegistry          *TaskRegistry
}

func (s *Service) Status() string {
	tasks := s.taskRegistry.GetAll()
	if len(tasks) == 0 {
		return taskOKStatus
	}
	for i, task := range tasks {
		if i > MaxStatusTaskCount {
			break
		}
		if task.Status == taskErrorStatus {
			return taskErrorStatus
		}
	}
	return taskOKStatus
}

func (s *Service) Start() error {
	running := atomic.LoadInt32(&s.isRunning)
	if running == 1 {
		return errors.New("service has been already started")
	}
	go func() {
		atomic.StoreInt32(&s.isRunning, 1)
		defer atomic.StoreInt32(&s.isRunning, 0)

		tick := time.NewTicker(time.Second).C
		for {
			select {
			case <-s.stopNotification:
				break
			case <-tick:
				err := s.Run()
				if err != nil {
					logger.Printf("Failed to Run status %v", err)
				}
			}
		}
	}()
	return nil
}

func (s *Service) Version() string {
	return Version
}

func (s *Service) Run() error {
	var result error
	if s.transferConfig == nil || len(s.transferConfig.Transfers) == 0 {
		return result
	}
	for _, transfer := range s.transferConfig.Transfers {
		go func(transfer *Transfer) {
			now := time.Now()
			if (transfer.nextRun == nil || transfer.nextRun.Before(now)) && !transfer.isRunning() {
				transfer.setRunning(true)
				defer transfer.setRunning(false)
				err := transfer.scheduleNextRun(now)
				if err != nil {
					logger.Printf("Failed to schedule Transfer: %v %v", err, transfer)
					result = err
					return
				}
				transferTask := NewTransferTask(transfer)
				s.taskRegistry.Register(transferTask.Task)
				err = s.transferService.Run(transferTask)
				if err != nil {
					logger.Printf("Failed to Transfer: %v %v", err, transfer)
					result = err
				}
				return
			}
		}(transfer)
	}
	return result
}

func (s *Service) GetTasks(request http.Request, ids ...string) []*Task {
	var result []*Task
	if len(ids) == 0 {
		result = s.taskRegistry.GetAll()
	} else {
		result = s.taskRegistry.GetByIDs(ids...)
	}
	request.ParseForm()
	offset := toolbox.AsInt(request.Form.Get("offset"))
	limit := toolbox.AsInt(request.Form.Get("limit"))
	if limit == 0 || limit > len(result) {
		limit = len(result)
	}
	return result[offset:limit]
}

func (s *Service) GetErrors() []*ObjectMeta {
	corruptedFiles := make([]*ObjectMeta, 0)
	//THIS WOULD NOT WORK

	//for _, transfer := range s.config.Transfers {
	//	meta, err  := s.transferService.LoadMeta(transfer.Meta)
	//	if err != nil {
	//		logger.Printf("Failed to load Meta file: %v %v", transfer, err)
	//		continue
	//	}
	//	for _, processedFile := range meta.ProcessedResources {
	//		if processedFile.Error != "" {
	//			corruptedFiles = append(corruptedFiles, processedFile)
	//		}
	//	}
	//}
	return corruptedFiles
}

func (s *Service) getMetaObject(name string, metaResource Resource) ([]*Meta, error) {
	parentUrlIndex := strings.LastIndex(metaResource.Name, "/")
	if parentUrlIndex == -1 {
		return nil, nil
	}
	var candidates = make([]storage.Object, 0)
	parentURL := expandCurrentWorkingDirectory(metaResource.Name[:parentUrlIndex])
	service, err := getStorageService(&metaResource)
	if err != nil {
		return nil, err
	}
	appendContentObject(service, parentURL, &candidates)
	var result = make([]*Meta, 0)
	for _, candidate := range candidates {
		if !strings.Contains(candidate.URL(), name) {
			continue
		}
		metaResource.Name = candidate.URL()
		meta, err := s.transferService.LoadMeta(&metaResource)
		if err != nil {
			return nil, err
		}
		result = append(result, meta)
	}
	return result, nil
}

func (s *Service) ProcessingStatus(name string) *StatusInfoResponse {
	var response = NewStatusInfoResponse()
	var metaResources = make([]*Resource, 0)
	for _, transfer := range s.transferConfig.Transfers {
		if strings.Contains(transfer.Meta.Name, name) {
			metaResources = append(metaResources, transfer.Meta)
		}
	}

	var metaUrl = make(map[string]bool)
	for _, metaResource := range metaResources {
		metaList, err := s.getMetaObject(name, *metaResource)
		if err != nil {
			response.Error = err.Error()
			return response
		}

		for _, meta := range metaList {
			if metaUrl[meta.URL] {
				continue
			}
			metaUrl[meta.URL] = true
			resourceStatus := NewResourceStatusInfo()
			resourceStatus.Resource = meta.URL
			resourceStatus.Errors = meta.Errors
			resourceStatus.Status = meta.Status
			resourceStatus.ResourceStatus = meta.ResourceStatus
			response.Status = append(response.Status, resourceStatus)
		}
	}
	toolbox.ReverseSlice(response.Status)
	return response
}

func (s *Service) Stop() {
	atomic.StoreInt32(&s.isRunning, 0)
	s.stopNotification <- true
}

func NewService(config *ServerConfig, transferConfig *TransferConfig) (*Service, error) {
	taskRegistry := NewTaskRegistry()
	var transferObjectService TransferObjectService
	if len(config.Cluster) == 0 {
		transferObjectService = newtransferObjectService(taskRegistry)
	} else {
		transferObjectService = newTransferObjectServiceClient(config.Cluster)
	}
	transferService := newTransferService(transferObjectService)
	var result = &Service{
		config:                config,
		transferConfig:        transferConfig,
		isRunning:             0,
		stopNotification:      make(chan bool, 1),
		transferService:       transferService,
		taskRegistry:          taskRegistry,
		transferObjectService: transferObjectService,
	}
	return result, nil
}
