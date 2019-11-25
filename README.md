## This tool provides Prometheus service discovery for ECS running on Alicloud.

### Usage:

```
prometheus-ali-sd ecs -l ecs -t cluster=prod --notagk "acs:autoscaling.*" --notagv autoScale --logfile /tmp/promsd.log --loglevel debug -o /tmp/promsd.json

Usage:
  prometheus-ali-sd ecs [flags]

Flags:
  -h, --help                  help for ecs
  -n, --instancename string   filter by instance name
  -l, --labelprefix string    Label prefix for ecs tag
      --notagk argList        filter by ecs instance tag key not contain keyword with regular expression example: acs:autoscaling.* (can specify multiple) (default [])
      --notagv argList        filter by ecs instance tag value not contain keyword with regular expression example: autoScale (can specify multiple) (default [])
  -o, --output string         file output path (default "/tmp/test.json")
  -s, --pagesize int          alicloud api pagesize (default 10)
      --regname argList       filter by ecs instance name with regular expression  example: ecs.* (can specify multiple, will use or operator) (default [])
  -t, --tag argList           filter by ecs instance tag example: cluster=prod (can specify multiple) (default [])

Global Flags:
      --config string     config file (default is $HOME/.prometheus-ali-sd.yaml)
      --logfile string    log file
      --loglevel string   log level  [trace, debug, info, warn, error, fatal, panic] (default info) (default "info")
```
