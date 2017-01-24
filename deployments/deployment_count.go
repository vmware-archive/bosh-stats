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

func (d *DeployCounter) SuccessfulDeploys(calendarMonth string) (int, error) {
	logger := boshlog.NewLogger(boshlog.LevelError)

	factory := boshuaa.NewFactory(logger)
	uaaConfig, err := boshuaa.NewConfigFromURL(d.UaaURL)
	if err != nil {
		return 0, err
	}
	uaaConfig.Client = d.UaaClientID
	uaaConfig.ClientSecret = d.UaaClientSecret
	uaaConfig.CACert = d.CaCert

	uaaClient, err := factory.New(uaaConfig)
	if err != nil {
		return 0, err
	}

	directorFactory := boshdir.NewFactory(logger)
	directorConfig, err := boshdir.NewConfigFromURL(d.DirectorURL)
	if err != nil {
		return 0, err
	}
	directorConfig.CACert = d.CaCert
	directorConfig.TokenFunc = boshuaa.NewClientTokenSession(uaaClient).TokenFunc
	directorClient, err := directorFactory.New(directorConfig, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	if err != nil {
		return 0, err
	}

	calendarMonthComponents := strings.Split(calendarMonth, "/")
	year, err := strconv.Atoi(calendarMonthComponents[0])
	if err != nil {
		return 0, err
	}
	month, err := strconv.Atoi(calendarMonthComponents[1])
	if err != nil {
		return 0, err
	}
	startTime := now.New(time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC))
	endTime := startTime.EndOfMonth()

	opts := boshdir.EventsFilter{
		Before: fmt.Sprintf("%d", endTime.Unix()),
		After:  fmt.Sprintf("%d", startTime.Unix()),
	}

	events, err := directorClient.Events(opts)
	if err != nil {
		return 0, err
	}

	successfulDeploys := 0

	for _, event := range events {
		if event.ObjectType() == "deployment" && (event.Action() == "create" || event.Action() == "update") && event.Error() == "" && len(event.Context()) > 0 {
			successfulDeploys++
		}
	}

	return successfulDeploys, nil
}
