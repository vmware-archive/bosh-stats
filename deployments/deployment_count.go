package deployments

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
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

func (d *DeployCounter) SuccessfulDeploys(calendarMonth string, itemsPerPage int, repaveUser string, runningCount *map[string]int) error {
	logger := boshlog.NewLogger(boshlog.LevelError)

	directorClient, err := createDirectorClient(d, logger)
	if err != nil {
		return err
	}

	opts, err := createCalendarOpts(calendarMonth)
	if err != nil {
		return err
	}

	err = reduceDeploymentsToCount(directorClient, []boshdir.Event{}, opts, itemsPerPage, runningCount, repaveUser)
	if err != nil {
		return err
	}

	return nil
}

func (d *DeployCounter) DeployDate(release string, version string, itemsPerPage int) (time.Time, error) {
	logger := boshlog.NewLogger(boshlog.LevelError)

	directorClient, err := createDirectorClient(d, logger)
	if err != nil {
		return time.Time{}, err
	}

	opts := boshdir.EventsFilter{}

	deployDate, err := reduceDeployDate(directorClient, []boshdir.Event{}, opts, itemsPerPage, release, version)
	if err != nil {
		return time.Time{}, err
	}
	return deployDate, err
}

func reduceDeployDate(directorClient boshdir.Director, events []boshdir.Event, opts boshdir.EventsFilter, itemsPerPage int, release string, version string) (time.Time, error) {
	newOpts := opts
	if len(events) != 0 {
		newOpts.BeforeID = events[len(events)-1].ID()
	}
	newEvents, err := directorClient.Events(newOpts)
	if err != nil {
		return time.Time{}, err
	}

	if len(newEvents) == 0 {
		return time.Time{}, errors.New(fmt.Sprintf("No events found for %s version %s", release, version))
	}

	date, found_ok := findDeployTime(newEvents, release, version)
	if found_ok {
		return date, nil
	} else {
		return reduceDeployDate(directorClient, newEvents, newOpts, itemsPerPage, release, version)
	}
}

func findDeployTime(events []boshdir.Event, release string, version string) (time.Time, bool) {
	for _, event := range events {
		if isDeployment(event) && IsReleaseUpdate(event, release, version) {
			return event.Timestamp(), true
		}
	}
	return time.Time{}, false
}

func reduceDeploymentsToCount(directorClient boshdir.Director, events []boshdir.Event, opts boshdir.EventsFilter, itemsPerPage int, runningCount *map[string]int, repaveUser string) error {
	if len(events) > 0 && len(events) < itemsPerPage {
		return nil
	}

	newOpts := opts
	if len(events) != 0 {
		newOpts.BeforeID = events[len(events)-1].ID()
	}
	newEvents, err := directorClient.Events(newOpts)
	if err != nil {
		return err
	}

	if len(newEvents) == 0 {
		return nil
	}

	deploymentEventCount(newEvents, runningCount, repaveUser)
	return reduceDeploymentsToCount(directorClient, newEvents, newOpts, itemsPerPage, runningCount, repaveUser)
}

func deploymentEventCount(events []boshdir.Event, runningCount *map[string]int, repaveUser string) {
	for _, event := range events {
		if isDeployment(event) && IsNotRepaveUser(event, repaveUser) {
			deploymentName := event.DeploymentName()
			(*runningCount)[deploymentName] += 1
		}
	}
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

func IsReleaseUpdate(event boshdir.Event, release string, version string) bool {
	context := event.Context()
	context_before, ok := context["before"].(map[string]interface{})

	if ok != true {
		return false
	}
	context_after, ok := context["after"].(map[string]interface{})

	if ok != true {
		return false
	}

	releases_before, ok := context_before["releases"].([]interface{})
	if ok != true {
		return false
	}

	releases_after, ok := context_after["releases"].([]interface{})
	if ok != true {
		return false
	}

	var version_before, version_after string
	var semver_before, semver_after, latest_semver_before semver.Version
	var err error

	re := regexp.MustCompile(fmt.Sprintf("^%s\\/(.*)", release))

	for _, release_before := range releases_before {
		matches := re.FindStringSubmatch(release_before.(string))
		if matches != nil {
			version_before = matches[1]
			semver_before, err = semver.Parse(version_before)
			if err != nil {
				semver_before, err = semver.Parse(version_before + ".0.0")
				if err != nil {
					panic(err)
				}
			}

			if latest_semver_before.LT(semver_before) {
				latest_semver_before = semver_before
			}
		}
	}

	for _, release_after := range releases_after {
		matches := re.FindStringSubmatch(release_after.(string))
		if matches != nil && matches[1] == version {
			version_after = matches[1]
			break
		}
	}

	if version_before == "" || version_after == "" {
		return false
	}

	semver_after, err = semver.Parse(version_after)
	if err != nil {
		semver_after, err = semver.Parse(version_after + ".0.0")
		if err != nil {
			panic(err)
		}
	}

	return semver_after.GT(latest_semver_before)
}

func isDeployment(event boshdir.Event) bool {
	return event.ObjectType() == "deployment" &&
		(event.Action() == "create" || event.Action() == "update") &&
		event.Error() == "" &&
		len(event.Context()) > 0
}
