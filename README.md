# bosh-stats
A tool to collect statistics from bosh director

## Assumptions:
1. This binary assumes that you have direct connectivity to BOSH and it's UAA. To test it, you can run `nc -v <bosh-director-ip> 25555` and `nc -v <uaa-ip> <uaa-port> (default: 8443)`

## To run this tool
1. Download the appropriate [binary](https://github.com/pivotal-cloudops/bosh-stats/releases/tag/1.0.0) for your environment.

All arguments are required
```
Usage of bosh-stats:
  -caCert string
      CA Cert
  -calendarMonth string
      Calendar month/year YYYY/MM
  -directorUrl string
      bosh director URL
  -json
      print JSON to standard out (output is a table by default)
  -repaveUser string
      The username to filter out as the 'repave' user
  -uaaClientId string
      UAA Client ID
  -uaaClientSecret string
      UAA Client Secret
  -uaaUrl string
      UAA URL
```


### Example:
```
bosh-stats \
   -uaaUrl https://<UAA_URL>:8443 \
   -uaaClientId bosh-stats \
   -uaaClientSecret yoursecrets \
   -directorUrl https://<BOSH_URL> \
   -caCert "$(cat <BOSH rootCA.pem>)" \
   -calendarMonth 2017/01
```
