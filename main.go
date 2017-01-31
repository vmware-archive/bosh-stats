package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/pivotal-cloudops/bosh-stats/deployments"
)

func printHeader(w *tabwriter.Writer) {
	fmt.Fprintln(w, "Deployment", "\t", "Count")
	fmt.Fprintln(w, "--------------------", "\t", "--------------------")
}

func friendlyCalendarMonth(calendarMonth *string) string {
	parsedMonth, err := time.Parse("2006/01", *calendarMonth)
	friendlyCalendarMonth := parsedMonth.Format("Jan 2006")

	if err != nil {
		friendlyCalendarMonth = *calendarMonth
	}

	return friendlyCalendarMonth
}

func printResults(numberByDeployment map[string]int, calendarMonth *string) {
	totalDeploys := 0
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)

	printHeader(w)

	for k, v := range numberByDeployment {
		totalDeploys += v
		fmt.Fprintln(w, k, "\t", v, "deploys")
	}

	fmt.Println()
	fmt.Fprintln(w, "--------------------", "\t", "--------------------")
	fmt.Fprintln(w, friendlyCalendarMonth(calendarMonth), "\t", totalDeploys, "total deploys")
	w.Flush()
}

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

	printResults(numberByDeployment, calendarMonth)
}
