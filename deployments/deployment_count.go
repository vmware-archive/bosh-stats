package deployments

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/jinzhu/now"
)

type DeployCounter struct {
	DirectorURL     string
	UaaURL          string
	UaaClientID     string
	UaaClientSecret string
	CaCert          string
}

func (d *DeployCounter) SuccessfulDeploys(calendarMonth string, itemsPerPage int) (int, error) {
	logger := boshlog.NewLogger(boshlog.LevelError)

	directorClient, err := createDirectorClient(d, logger)
	if err != nil {
		return 0, err
	}

	opts, err := createCalendarOpts(calendarMonth)
	if err != nil {
		return 0, err
	}

	successfulDeploys, err := reduceDeploymentsToCount(directorClient, []boshdir.Event{}, opts, itemsPerPage, 0)
	if err != nil {
		return successfulDeploys, err
	}

	return successfulDeploys, nil
}

func reduceDeploymentsToCount(directorClient boshdir.Director, events []boshdir.Event, opts boshdir.EventsFilter, itemsPerPage int, runningCount int) (int, error) {
	if len(events) > 0 && len(events) < itemsPerPage {
		return runningCount, nil
	}

	newOpts := opts
	if len(events) != 0 {
		newOpts.BeforeID = events[len(events)-1].ID()
	}
	newEvents, err := directorClient.Events(newOpts)
	if err != nil {
		return 0, err
	}

	return reduceDeploymentsToCount(directorClient, newEvents, newOpts, itemsPerPage, runningCount+deploymentEventCount(newEvents))
}

func deploymentEventCount(events []boshdir.Event) int {
	successfulDeploys := 0
	for _, event := range events {
		if isDeployment(event) && IsNotRepaveUser(event, "repave") {
			successfulDeploys++
		}
	}
	return successfulDeploys
}

func createDirectorClient(d *DeployCounter, logger boshlog.Logger) (boshdir.Director, error) {
	uaaClient, err := createUaaClient(d, logger)
	if err != nil {
		return nil, err
	}

	directorFactory := boshdir.NewFactory(logger)
	directorConfig, err := boshdir.NewConfigFromURL(d.DirectorURL)
	if err != nil {
		return nil, err
	}
	directorConfig.CACert = d.CaCert
	directorConfig.TokenFunc = boshuaa.NewClientTokenSession(uaaClient).TokenFunc
	directorClient, err := directorFactory.New(directorConfig, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	if err != nil {
		return nil, err
	}
	return directorClient, nil
}

func createUaaClient(d *DeployCounter, logger boshlog.Logger) (boshuaa.UAA, error) {
	factory := boshuaa.NewFactory(logger)
	uaaConfig, err := boshuaa.NewConfigFromURL(d.UaaURL)
	if err != nil {
		return nil, err
	}

	uaaConfig.Client = d.UaaClientID
	uaaConfig.ClientSecret = d.UaaClientSecret
	uaaConfig.CACert = d.CaCert

	uaaClient, err := factory.New(uaaConfig)
	if err != nil {
		return nil, err
	}

	return uaaClient, nil
}

func createCalendarOpts(calendarMonth string) (boshdir.EventsFilter, error) {
	calendarMonthComponents := strings.Split(calendarMonth, "/")
	year, err := strconv.Atoi(calendarMonthComponents[0])
	if err != nil {
		return boshdir.EventsFilter{}, err
	}
	month, err := strconv.Atoi(calendarMonthComponents[1])
	if err != nil {
		return boshdir.EventsFilter{}, err
	}
	startTime := now.New(time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC))
	endTime := startTime.EndOfMonth()

	opts := boshdir.EventsFilter{
		Before: fmt.Sprintf("%d", endTime.Unix()),
		After:  fmt.Sprintf("%d", startTime.Unix()),
	}

	return opts, nil
}

func IsNotRepaveUser(event boshdir.Event, repaveUser string) bool {
	if event.User() == repaveUser {
		return false
	}

	return true
}

func isDeployment(event boshdir.Event) bool {
	return event.ObjectType() == "deployment" &&
		(event.Action() == "create" || event.Action() == "update") &&
		event.Error() == "" &&
		len(event.Context()) > 0
}
