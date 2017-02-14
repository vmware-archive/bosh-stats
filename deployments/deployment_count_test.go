package deployments_test

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/cloudfoundry/bosh-cli/director/directorfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cloudops/bosh-stats/deployments"
)

var _ = Describe("counting bosh deployments and get deploy date", func() {
	var (
		uaa      *ghttp.Server
		director *ghttp.Server
	)

	startHttpsServer := func(cert, key string) *ghttp.Server {
		server := ghttp.NewUnstartedServer()
		keypair, err := tls.X509KeyPair([]byte(cert), []byte(key))
		Expect(err).NotTo(HaveOccurred())
		server.HTTPTestServer.TLS = &tls.Config{
			Certificates: []tls.Certificate{keypair},
		}
		server.HTTPTestServer.StartTLS()
		return server
	}

	BeforeEach(func() {
		director = startHttpsServer(validCert, validKey)
		uaa = startHttpsServer(validCert, validKey)
	})

	AfterEach(func() {
		director.Close()
		uaa.Close()
	})

	Context("no page", func() {
		statusOK := http.StatusOK
		events := `[]`

		BeforeEach(func() {
			token := map[string]string{"token": "itsatoken"}

			uaa.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token"),
				ghttp.VerifyBasicAuth("some-client", "itsasecret"),
				ghttp.RespondWithJSONEncodedPtr(&statusOK, &token),
			))

		})

		It("returns 0 when no events are found", func() {
			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_time=1448927999&after_time=1446336000"),
					ghttp.RespondWith(statusOK, events),
				),
			)

			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			runningCount := make(map[string]int)
			err := deployCounter.SuccessfulDeploys("2015/11", 999, "repave", &runningCount)
			Expect(director.ReceivedRequests()).To(HaveLen(1))
			Expect(uaa.ReceivedRequests()).To(HaveLen(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(runningCount).To(Equal(map[string]int{}))

		})

		It("raise error when no events releated to the release deploy", func() {
			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events"),
					ghttp.RespondWith(statusOK, events),
				),
			)

			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			date, err := deployCounter.DeployDate("cf", "123", 999)
			Expect(director.ReceivedRequests()).To(HaveLen(1))
			Expect(uaa.ReceivedRequests()).To(HaveLen(1))
			Expect(err).To(HaveOccurred())
			Expect(date).To(Equal(time.Time{}))
		})
	})

	Context("one page", func() {
		BeforeEach(func() {
			statusOK := http.StatusOK
			token := map[string]string{"token": "itsatoken"}
			events := `
			[
				{
					"id": "6",
					"action": "create",
					"error": "",
					"object_type": "deployment",
					"object_name": "depl1_that_shouldnt_be_counted_with_no_context",
					"deployment": "bla1",
					"task": "6"
				},
				{
					"id": "5",
					"action": "create",
					"error": "",
					"object_type": "deployment",
					"object_name": "depl1",
					"deployment": "bla1",
					"task": "6",
					"context": {"new name": "depl2"}
				},
				{
					"id": "4",
					"action": "create",
					"error": "didn't go well",
					"object_type": "deployment",
					"object_name": "failed_deployment",
					"deployment": "bla1",
					"task": "7",
					"context": {"new name": "depl2"}
				},
				{
					"id": "3",
					"action": "delete",
					"error": "",
					"object_type": "deployment",
					"object_name": "depl1",
					"deployment": "bla2",
					"task": "8",
					"context": {"new name": "depl2"}
				},
				{
					"id": "2",
					"action": "update",
					"timestamp": 1448000000,
					"error": "",
					"object_type": "deployment",
					"object_name": "depl1",
					"deployment": "bla2",
					"task": "9",
					"context": {
						"before": {
							"releases": ["cf/122"],
							"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
						},
						"after": {
							"releases": ["cf/123"],
							"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
						}
					}
				},
				{
					"id": "1",
					"action": "create",
					"error": "",
					"object_type": "spleloymnt",
					"object_name": "depl1",
					"deployment": "bla1",
					"task": "9",
					"context": {"new name": "depl2"}
				}
			]`

			uaa.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token"),
				ghttp.VerifyBasicAuth("some-client", "itsasecret"),
				ghttp.RespondWithJSONEncodedPtr(&statusOK, &token),
			))

			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events"),
					ghttp.RespondWith(statusOK, events),
				),
			)
		})

		It("returns the number of successful deploys in the provided month", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			runningCount := make(map[string]int)
			expectedRunningcount := map[string]int{
				"bla1": 1,
				"bla2": 1,
			}
			err := deployCounter.SuccessfulDeploys("2015/11", 999, "repave", &runningCount)
			Expect(director.ReceivedRequests()).To(HaveLen(1))
			Expect(uaa.ReceivedRequests()).To(HaveLen(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(runningCount).To(Equal(expectedRunningcount))
		})

		It("returns the date of deploy happens to update the release", func() {
			statusOK := http.StatusOK
			events := `[]`

			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_id=1"),
					ghttp.RespondWith(statusOK, events),
				),
			)

			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			date, err := deployCounter.DeployDate("cf", "123", 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(date).To(Equal(time.Unix(1448000000, 0).UTC()))
		})

		It("raises an error when there is no release update coresponded to the version", func() {
			statusOK := http.StatusOK
			events := `[]`

			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_id=1"),
					ghttp.RespondWith(statusOK, events),
				),
			)

			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			_, err := deployCounter.DeployDate("diego", "1.6.2", 999)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("No events found for diego version 1.6.2"))
		})

	})

	Context("many pages", func() {
		statusOK := http.StatusOK
		eventsPage1 := `
		[
		{
			"id": "4",
			"action": "create",
			"error": "",
			"user": "not-repave",
			"object_type": "deployment",
			"object_name": "depl1",
			"deployment": "bla1",
			"task": "6",
			"context": {"new name": "depl1"}
		},
		{
			"id": "3",
			"action": "create",
			"error": "FAAAAAAAAAAILED",
			"user": "not-repave",
			"object_type": "deployment",
			"object_name": "failed_deployment",
			"deployment": "bla1",
			"task": "6"
		},
		{
			"id": "2",
			"action": "create",
			"error": "",
			"user": "MyCustomRepaveUserInProd",
			"object_type": "deployment",
			"object_name": "",
			"deployment": "bla1",
			"task": "6",
			"context": {"new name": "depl1"}
		}
		]`

		eventsPage2 := `
		[
		{
			"id": "1",
			"action": "update",
			"timestamp": 1448000000,
			"error": "",
			"user": "not-repave",
			"object_type": "deployment",
			"object_name": "depl1",
			"deployment": "bla2",
			"task": "7",
			"context": {
				"before": {
					"releases": ["cf/122"],
					"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
				},
				"after": {
					"releases": ["cf/122", "cf/123"],
					"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
				}
			}
		}
		]`

		eventsPage2_diego := `
		[
		{
			"id": "1",
			"action": "update",
			"timestamp": 1448000000,
			"error": "",
			"user": "not-repave",
			"object_type": "deployment",
			"object_name": "depl1",
			"deployment": "bla2",
			"task": "7",
			"context": {
				"before": {
					"releases": ["cf/123","diego/1.6.0"],
					"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
				},
				"after": {
					"releases": ["cf/123", "diego/1.6.2"],
					"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
				}
			}
		}
		]`

		eventsPage2_foo := `
		[
		{
			"id": "1",
			"action": "update",
			"timestamp": 1448000000,
			"error": "",
			"user": "not-repave",
			"object_type": "deployment",
			"object_name": "depl1",
			"deployment": "bla2",
			"task": "7",
			"context": {
				"before": {
					"releases": ["foo/123","foo/124"],
					"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
				},
				"after": {
					"releases": ["foo/123", "foo/124"],
					"stemcells": ["bosh-aws-xen-hvm-ubuntu-trusty-go_agent/3312.12"]
				}
			}
		}
		]`

		eventsPage3_foo := `[]`

		BeforeEach(func() {
			token := map[string]string{"token": "itsatoken"}

			uaa.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token"),
				ghttp.VerifyBasicAuth("some-client", "itsasecret"),
				ghttp.RespondWithJSONEncodedPtr(&statusOK, &token),
			))
		})

		It("returns the number of successful deploys in the provided month", func() {
			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_time=1448927999&after_time=1446336000"),
					ghttp.RespondWith(statusOK, eventsPage1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "after_time=1446336000&before_id=2&before_time=1448927999"),
					ghttp.RespondWith(statusOK, eventsPage2),
				),
			)

			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}
			runningCount := make(map[string]int)
			expectedRunningcount := map[string]int{
				"bla1": 2,
				"bla2": 1,
			}
			err := deployCounter.SuccessfulDeploys("2015/11", 3, "repave", &runningCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(runningCount).To(Equal(expectedRunningcount))
		})

		It("filters out deployments made by the repave user given", func() {
			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_time=1448927999&after_time=1446336000"),
					ghttp.RespondWith(statusOK, eventsPage1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "after_time=1446336000&before_id=2&before_time=1448927999"),
					ghttp.RespondWith(statusOK, eventsPage2),
				),
			)

			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}
			runningCount := make(map[string]int)
			expectedRunningcount := map[string]int{
				"bla1": 1,
				"bla2": 1,
			}

			err := deployCounter.SuccessfulDeploys("2015/11", 3, "MyCustomRepaveUserInProd", &runningCount)
			Expect(director.ReceivedRequests()).To(HaveLen(2))
			Expect(err).NotTo(HaveOccurred())
			Expect(runningCount).To(Equal(expectedRunningcount))
		})

		It("find the deploy date of cf/123", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events"),
					ghttp.RespondWith(statusOK, eventsPage1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_id=2"),
					ghttp.RespondWith(statusOK, eventsPage2),
				),
			)

			date, err := deployCounter.DeployDate("cf", "123", 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(date).To(Equal(time.Unix(1448000000, 0).UTC()))
		})

		It("find the deploy date of diego/1.6.2", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events"),
					ghttp.RespondWith(statusOK, eventsPage1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_id=2"),
					ghttp.RespondWith(statusOK, eventsPage2_diego),
				),
			)

			date, err := deployCounter.DeployDate("diego", "1.6.2", 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(date).To(Equal(time.Unix(1448000000, 0).UTC()))
		})

		It("raises an error finding the deploy date of foo/124", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events"),
					ghttp.RespondWith(statusOK, eventsPage1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_id=2"),
					ghttp.RespondWith(statusOK, eventsPage2_foo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_id=1"),
					ghttp.RespondWith(statusOK, eventsPage3_foo),
				),
			)

			_, err := deployCounter.DeployDate("foo", "124", 3)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("No events found for foo version 124"))
		})

	})

	Context("error returned from director", func() {
		BeforeEach(func() {
			statusError := http.StatusInternalServerError
			statusOK := http.StatusOK
			token := map[string]string{"token": "itsatoken"}

			uaa.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token"),
					ghttp.VerifyBasicAuth("some-client", "itsasecret"),
					ghttp.RespondWithJSONEncodedPtr(&statusOK, &token),
				),
			)

			director.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/events", "before_time=1448927999&after_time=1446336000"),
					ghttp.RespondWith(statusError, "sorry bro"),
				),
			)
		})

		It("returns a 0 count and an error", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}
			runningCount := make(map[string]int)
			expectedRunningcount := map[string]int{}

			err := deployCounter.SuccessfulDeploys("2015/11", 999, "repave", &runningCount)
			Expect(director.ReceivedRequests()).To(HaveLen(1))
			Expect(err).To(HaveOccurred())
			Expect(runningCount).To(Equal(expectedRunningcount))
		})
	})

	Context("using a bad UAA URL", func() {
		It("returns a 0 count and an error", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          "%%%",
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}
			runningCount := make(map[string]int)
			expectedRunningcount := map[string]int{}

			err := deployCounter.SuccessfulDeploys("2015/11", 999, "repave", &runningCount)
			Expect(director.ReceivedRequests()).To(HaveLen(0))
			Expect(err).To(HaveOccurred())
			Expect(runningCount).To(Equal(expectedRunningcount))
		})
	})

	Context("using a bad director URL", func() {
		It("returns a 0 count and an error", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     "",
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			runningCount := make(map[string]int)
			expectedRunningcount := map[string]int{}

			err := deployCounter.SuccessfulDeploys("2015/11", 999, "repave", &runningCount)
			Expect(director.ReceivedRequests()).To(HaveLen(0))
			Expect(err).To(HaveOccurred())
			Expect(runningCount).To(Equal(expectedRunningcount))
		})
	})

	Context("error returned from UAA", func() {
		BeforeEach(func() {
			statusError := http.StatusInternalServerError
			token := map[string]string{"token": "itsatoken"}

			uaa.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token"),
					ghttp.VerifyBasicAuth("some-client", "itsasecret"),
					ghttp.RespondWithJSONEncodedPtr(&statusError, &token),
				),
			)
		})

		It("returns a 0 count and an error", func() {
			deployCounter := &deployments.DeployCounter{
				DirectorURL:     director.URL(),
				UaaURL:          uaa.URL(),
				UaaClientID:     "some-client",
				UaaClientSecret: "itsasecret",
				CaCert:          validCACert,
			}

			runningCount := make(map[string]int)
			expectedRunningcount := map[string]int{}

			err := deployCounter.SuccessfulDeploys("2015/11", 999, "repave", &runningCount)
			Expect(director.ReceivedRequests()).To(HaveLen(0))
			Expect(err).To(HaveOccurred())
			Expect(runningCount).To(Equal(expectedRunningcount))
		})
	})
})

var _ = Describe("#isNotRepaveUser", func() {
	It("returns true if the event's user is not the repave user", func() {
		var event = new(directorfakes.FakeEvent)
		event.UserReturns("not-repave")

		repaveUser := deployments.IsNotRepaveUser(event, "repave")
		Expect(repaveUser).To(Equal(true))
	})

	It("returns false if the event's user is the repave user", func() {
		var event = new(directorfakes.FakeEvent)
		event.UserReturns("repave")

		repaveUser := deployments.IsNotRepaveUser(event, "repave")
		Expect(repaveUser).To(Equal(false))
	})
})

var validCert = `-----BEGIN CERTIFICATE-----
MIIDDTCCAfWgAwIBAgIJAOYPl1HNpMPsMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwIBcNMTYwMTE2MDY0NTA0WhgPMjI4OTEwMzAwNjQ1MDRa
MDAxCzAJBgNVBAYTAlVTMQ0wCwYDVQQKDARCT1NIMRIwEAYDVQQDDAkxMjcuMC4w
LjEwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDlptk3/IXbiBgJO7DO
dSc9MASV7FSBATumxQcXvKzUuaBJECD/S/QdevoBtIXQhtyNdSNu8GN6cD550xs2
3DYibgPD+At1IxRHfGu0Hxn2ZbU4yP9SqUchJHOa7Rix6T2cnauYhh+FhilO0Elm
kOyOtAshnv70ZWUDez8ybExgSK2kCiq3tmFotNHpxN6gNJ9IQfYz1U3thX/kyjag
MrOTTzluGGgpyS7o+4eD5rL/pWTylkgufhqUm4CJkRbXlJ8Dd/bwuBtRTumO6C4q
sYU6/OGQT/HM+sYDzrUd2pe36dQ41oeWZhKn2DyixnLLqlcH3QxnHTeg139sIQfy
rIMPAgMBAAGjEzARMA8GA1UdEQQIMAaHBH8AAAEwDQYJKoZIhvcNAQEFBQADggEB
AKj2aCf1tLQFZLq+TYa/THoVP7Pmwnt49ViQO8nMnfCM3459d52vCdIodGocVg9T
x8N4/pIG3S0VCzQt+4+UJej6SyecrYpcMCtWhZ73zxTJ7lQUmknsqZCvC5BcjYgF
McML3CeFsHuHvwb7uH5h8VO6UWyFTj7xNsH4E3XZT3I92fdS11pfrBSJDGfkiAQ/
j3N1QevrxTlEuKLQFfFSbnA3XZGpkDzg/sqYiOHnVgbn84IIZ3lGXs+qzC5kTFfM
SC0K79vs7peS+FdzPUAuG7uyy0W0s5hFTRIlcvBO5w9QrwEnBEv7WrZ6oSZ5F3Ku
/M/AnjGop4LUFIbJQR0ns7U=
-----END CERTIFICATE-----`

var validKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA5abZN/yF24gYCTuwznUnPTAElexUgQE7psUHF7ys1LmgSRAg
/0v0HXr6AbSF0IbcjXUjbvBjenA+edMbNtw2Im4Dw/gLdSMUR3xrtB8Z9mW1OMj/
UqlHISRzmu0Ysek9nJ2rmIYfhYYpTtBJZpDsjrQLIZ7+9GVlA3s/MmxMYEitpAoq
t7ZhaLTR6cTeoDSfSEH2M9VN7YV/5Mo2oDKzk085bhhoKcku6PuHg+ay/6Vk8pZI
Ln4alJuAiZEW15SfA3f28LgbUU7pjuguKrGFOvzhkE/xzPrGA861HdqXt+nUONaH
lmYSp9g8osZyy6pXB90MZx03oNd/bCEH8qyDDwIDAQABAoIBAH82/f1VlZEWwrna
pwa3PxVWFDQ4xlbwJ+sqGdO8YME2UuQmWyERIhlylit7pOTu0B5MVWSPJYwdwX4a
w2iQdCx+ZPeZ4D7zP7iZ48/Tqr4jeVALh+RygUSKvL+Ft7hWTBsF/JhxM+TzfM57
8y0t+tzSP5hQS0t3H43eKBP2ihiLHVQwSV8F3GTNh2/yc3+Y+usO4n5T38Q6nC8K
OblPkR1riLMReMZRdhDvIox1OwZC00PH/2tJP/vLAHXxxGuD7Bo7BjuOND3aZ9Du
xi998w1B4LrRI/W9X53Q0q+GEGwGbuvEl7GDlihiucse7GzO6+Q0qk5bNvlmd0jL
EW3hGQECgYEA+5tznyQYxZQOhSpuISY3MbZ+SL1fZzmpnQviegWJg8dZTrUxLOmK
ku4Gyr+S+kA/tgK6ys4qRlzF2UCrytXGwoQuOxkK81rbhHqmZEZupLd5PXyAtZpz
AySUK/YLrtmXZP+gvOhO2ss9jMD8Nwy8nMa3hZDBomE63sHhFwgHhMcCgYEA6alE
ieOdMJJZ8ZpDMnYtwXuU31ne9Gj7lkirbSZ/l/SZ69xCMp8O6Oj66w5l8lH42lP3
cqJ+n35F0/TwKvft9nXCrc5H1zUw5qKYgDqn80bHk2cPqrenikSK3RhKHW4gJm+R
J9SY6bPPz/CCMqMX+cC/bjhdH/2gkldra2GoN3kCgYEA3bM9LwXsefQa00Xu4nC9
A6XtIoUTEm7hwIrfVWuZny9BxzOrEAr82ri37WDeznlcajF/jAIbiAJpJyRv+3tg
9rbn0ZUgbAwsD1DPWt4g0i0EvKP++YYNP8C0ewQDiV8bopgId0wvZ2TcaDEITC2B
6JbE0QEbTcxkxjGJ9/RQQ7MCgYEA3B681ZWaoIZOuz8S7LfONQah4aM9WUyJLjN5
YxMwgktIsZxGtH+JQTsyHjvrKFO2tp8BbnnMBZ6kU5/cnO4Bu/uGEcxRe1i9n5gv
SCV50MGuA5vEc5Qd/jDCDLT0JTN4kBzsRvSNtSPSstalIOTqEjtVW5U3jYqWOSan
qHpQSSkCgYBkXiIhH7rPd5oNYccNEzEr+0ew6HqzP8AkDSLTP323JK9kGw+dvEb/
dEG/RBqYUo0MiCCNXOVOsri1tL5cKZEfWgcTyzbkX/7BgHMWHkAD5QnhXaik5NZN
nLpUTgSaa9Cd6yjEW4wGyls8DxPHonM3XDSFc15VX1VFQiwbZBxQiw==
-----END RSA PRIVATE KEY-----`

var validCACert = `-----BEGIN CERTIFICATE-----
MIIDXzCCAkegAwIBAgIJAPerMgLAne5vMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwIBcNMTYwMTE2MDY0NTA0WhgPMjI4OTEwMzAwNjQ1MDRa
MEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJ
bnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw
ggEKAoIBAQCtSo3KPjnVPzodb6+mNwbCdcpzVop8OmfwJ3ynQtyBEzGaKsAn4tlz
/wfQQrKFHgxqVpqcoxAlWPNMs5+iO2Jst3Gz2+oLcaDyz/EWorw0iF5q1F6+WYHp
EijY20MzaWYMyu4UhhlbJCkSGZSjujh5SFOAXQwWYJXsqjyxA9KaTD6OdH5Kpger
B9D4zogX0We00eouyvvz/sAeDbTshk9sJRGWHNFJr+TjVx2D01alU49liAL94yF6
1eEOEbE50OAhv9RNsRh6O58idaHg30bbMf1yAzcgBvh8CzIHH0BPofoF2pRfztoY
uudZ0ftJjTz4fA2h/7GOVzxemrTjx88vAgMBAAGjUDBOMB0GA1UdDgQWBBQjz5Q2
YW2kBTb4XLqKFZMSBLpi6zAfBgNVHSMEGDAWgBQjz5Q2YW2kBTb4XLqKFZMSBLpi
6zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3DQEBBQUAA4IBAQA/s94M/mSGELHJWIb1
oE0IKHWajBd3Pc8+O1TZRE+ke3q+rZRfcxd2dAjq6zQHJUs2+fs0B3DyT9Wtyyoq
UrRdsgprOdf2Cuw8bMIsCQOvqWKhhdlLTnCi2xaGJawGsIkheuD1n+Il9gRQ2WGy
lACxVngPwjNYxjOE+CUnSZCuAmAfQYzqto3bNPqkgEwb7ueODeOiyhR8SKsH7ySW
QAOSxgrLBblGLWcDF9fjMeYaUnI34pHviCKeVxfgsxDR+Jg11F78sPdYLOF6ipBe
/5qTYucsY20B2EKtlscD0mSYBRwbVrSQt2RYbTCwaibxWUC13VV+YEk0NAv9Mm04
6sKO
-----END CERTIFICATE-----`
