package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pivotal-cloudops/bosh-stats/deployments"
)

func main() {
	directorURL := flag.String("directorUrl", "", "bosh director URL")
	uaaURL := flag.String("uaaUrl", "", "UAA URL")
	uaaClientID := flag.String("uaaClientId", "", "UAA Client ID")
	uaaClientSecret := os.Getenv("BOSH_STATS_UAA_CLIENT_SECRET")
	caCert := flag.String("caCert", "", "CA Cert")
	calendarMonth := flag.String("calendarMonth", "", "Calendar month/year YYYY/MM")
	flag.Parse()

	deployCounter := deployments.DeployCounter{
		DirectorURL:     *directorURL,
		UaaURL:          *uaaURL,
		UaaClientID:     *uaaClientID,
		UaaClientSecret: uaaClientSecret,
		CaCert:          *caCert,
	}

	numberOfDeploys, err := deployCounter.SuccessfulDeploys(*calendarMonth, 200)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(numberOfDeploys)
}
