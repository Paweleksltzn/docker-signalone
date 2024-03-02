package jobs

import (
	"context"
	"fmt"
	"regexp"
	"signal/helpers"
	"signal/models"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

func ScanForErrors(dockerClient *client.Client, logger *logrus.Logger, taskPayload models.TaskPayload, containersState map[string]*time.Time) {
	var currentsIDs = make([]string, 0)
	containers, err := helpers.ListContainers(dockerClient)
	if err != nil {
		logger.Errorf("Failed to list containers: %v", err)
		return
	}
	wg := sync.WaitGroup{}
	for _, c := range containers {
		currentsIDs = append(currentsIDs, c.ID)
		_, exists := containersState[c.ID]
		if !exists {
			containerCreationTime := time.Unix(c.Created, 0)
			containersState[c.ID] = &containerCreationTime
		}

		wg.Add(1)
		go func(dockerClient *client.Client,
			c types.Container, l *logrus.Logger,
			wg *sync.WaitGroup, taskPayload models.TaskPayload) {
			var timestampCheckpoint time.Time
			isErrorState := false
			logString := ""
			defer wg.Done()
			container, err := dockerClient.ContainerInspect(context.Background(), c.ID)
			if err != nil {
				l.Errorf("Failed to inspect container %s: %v", c.ID, err)
				return
			}

			logs, err := helpers.CollectLogsForAnalysis(c.ID, dockerClient)
			if err != nil {
				l.Errorf("Failed to collect logs for container %s: %v", c.ID, err)
			}

			for _, log := range logs {
				if log.Timestamp.After(*containersState[c.ID]) {
					logString += (log.Log + "\n")
					timestampCheckpoint = log.Timestamp
				}
			}
			logger.Infof("Scanning container %s, LOGS FOR SCANNING: \n %s", c.ID, logString)
			if logString != "" {
				containersState[c.ID] = &timestampCheckpoint
			}
			isErrorState = isContainerInErrorState(container.State)
			if isErrorState && logString != "" {
				// err := helpers.CallLogAnalysis(logString, c.Names[0], taskPayload)
				// if err != nil {
				// 	l.Errorf("Failed to call log analysis for container %s: %v", c.Names[0], err)
				// }
				return
			}
			isErrorState = areLogsIndicatingErrorOrWarning(logString)
			if isErrorState {
				// err := helpers.CallLogAnalysis(logString, c.Names[0], taskPayload)
				// if err != nil {
				// 	l.Errorf("Failed to call log analysis for container %s: %v", c.Names[0], err)
				// }
				return
			}
		}(dockerClient, c, logger, &wg, taskPayload)
	}

	wg.Wait()
	for k, _ := range containersState {
		fmt.Print(k)
		if wasContainerDeleted(k, currentsIDs) {
			delete(containersState, k)
		}
	}
}

func isContainerInErrorState(state *types.ContainerState) bool {
	return (state.Error != "" ||
		(!state.Running && state.ExitCode != 0))
}

func areLogsIndicatingErrorOrWarning(logs string) bool {
	regexWarningError := `(?i)(abort|blocked|corrupt|crash|critical|deadlock|denied|
		err|error|exception|fatal|forbidden|freeze|hang|illegal|invalid|issue|missing|
		panic|rejected|refused|stacktrace|timeout|traceback|unauthorized|uncaught|unexpected|unhandled|
		unimplemented|unsupported|warn|warning)`
	matched, _ := regexp.MatchString(regexWarningError, strings.ToLower(logs))
	return matched
}

func wasContainerDeleted(k string, currentsIDs []string) bool {
	for _, c := range currentsIDs {
		if c == k {
			return false
		}
	}
	return true
}
