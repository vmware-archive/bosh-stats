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
	uaaClientSecret := flag.String("uaaClientSecret", "", "UAA Client Secret")
	caCert := flag.String("caCert", "", "CA Cert")
	calendarMonth := flag.String("calendarMonth", "", "Calendar month/year YYYY/MM")
	repaveUser := flag.String("repaveUser", "", "The username to filter out as the 'repave' user")
	flag.Parse()

	deployCounter := deployments.DeployCounter{
		DirectorURL:     *directorURL,
		UaaURL:          *uaaURL,
		UaaClientID:     *uaaClientID,
		UaaClientSecret: *uaaClientSecret,
		CaCert:          *caCert,
	}

	numberByDeployment := make(map[string]int)
	err := deployCounter.SuccessfulDeploys(*calendarMonth, 200, *repaveUser, &numberByDeployment)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Success deploys by deployment name: \n %v\n", numberByDeployment)
}
