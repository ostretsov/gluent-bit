# gluent-bit
Kubernetes pod logs ↣ Graylog

[![](https://images.microbadger.com/badges/image/ostretsov/gluent-bit.svg)](https://microbadger.com/images/ostretsov/gluent-bit "Get your own image badge on microbadger.com")

Use `gluent-bit` if you just want to forward pod logs to Graylog. It's simple, lightweight and written in Go. 

## Getting started
```shell script
$ kubectl create namespace logging
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-service-account.yaml
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-role.yaml
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-role-binding.yaml
$ # and finally deploy DaemonSet
$ # change environment vars GRAYLOG_HOST and GRAYLOG_PORT if needed
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-ds.yaml
```

All you need then is to annotate a pod with `logging: "enabled"` and get logs forwarded into Graylog.

You could check if `gluent-bit` works:
```shell script
$ kubectl create namespace debug
$ # the following pod write text in stdout every 5 seconds
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/debug-infinite-log.yaml
$ # check if logs are in Graylog
$ # if everything is working delete debug namespace
$ kubectl delete namespace debug
```