Netgear Genie Stats Collector
=============================

I wrote this little utility to track the stats on my Home Router (Nighthawk R800) and report to my TICK stack.

Compile to a file (e.g. /opt/collectors/netgear/netgear-genie-stats)


Sample ENV file (e.g. /opt/collectors/netgear/.env)

```env
ROUTER_USER=admin
ROUTER_PASS=password
ROUTER_ADDR=192.168.1.1
```

Run as normal.

Sample configuration for Telegraf in `/etc/telegraf/telegraf.d/router.conf`

```
[[inputs.exec]]
  commands = ["/opt/collectors/netgear/netgear-genie-stats"]
  timeout = "5s"
  data_format = "influx"
  name_suffix = "_netgear_r8000"
```

