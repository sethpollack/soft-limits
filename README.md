# kubernetes Soft-Limits

The soft-limit controller allows you to set soft resource limits via pod annotations and will kill the pods gracefully when they exceed those limits.

Create the controller with `kubectl apply -f https://raw.githubusercontent.com/sethpollack/soft-limits/master/example.yaml`

Add the following annotations `sethpollack.net/soft-limit-cpu` and `sethpollack.net/soft-limit-memory` to your pods to set the soft limits.
