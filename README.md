# gluent-bit
Kubernetes pod logs ↣ Graylog

[![](https://images.microbadger.com/badges/image/ostretsov/gluent-bit.svg)](https://microbadger.com/images/ostretsov/gluent-bit "Get your own image badge on microbadger.com")

## Getting started
```shell script
$ kubectl create namespace logging
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-service-account.yaml
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-role.yaml
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-role-binding.yaml
$ # and finally deploy DaemonSet
$ kubectl apply -f https://raw.githubusercontent.com/ostretsov/gluent-bit/master/kubernetes/gluent-bit-ds.yaml
```