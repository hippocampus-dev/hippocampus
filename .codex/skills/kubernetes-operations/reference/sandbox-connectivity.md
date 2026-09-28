# Reaching the Cluster from a Claudex Sandbox

`minikube status`, `minikube ssh`, and `minikube update-context` all query the VM's state through libvirt first and fail with `GUEST_STATUS: ... failed connecting to libvirt socket: ... Permission denied` in a claudex sandbox, which cannot reach `/var/run/libvirt/virtqemud-sock`. This blocks the usual fix for a `~/.kube/config` whose context has no matching `clusters`/`users` entries. Reach the node directly instead.

## Get the node IP

```
jq -r '.Nodes[0].IP' ~/.minikube/profiles/minikube/config.json
```

## SSH to the node

Plain `ssh` fails in this sandbox two ways, both independent of minikube:

| Symptom | Cause | Workaround |
|---------|-------|------------|
| `No user exists for uid 1000` (fails before any network activity, even with `-v`) | `getpwuid()` on the local uid has no `/etc/passwd` entry | `LD_PRELOAD` a tiny shared library overriding `getpwuid`/`getpwuid_r`/`getpwnam` to return a fake entry |
| `Bad owner or permissions on /etc/ssh/ssh_config.d/20-systemd-ssh-proxy.conf` | That file is a symlink owned by `nobody` in this sandbox's user namespace, unconditionally `Include`d from `/etc/ssh/ssh_config` | `-F /dev/null` |

```
ssh -F /dev/null -o StrictHostKeyChecking=no -o UserKnownHostsFile=/tmp/known_hosts \
  -i ~/.minikube/machines/minikube/id_rsa docker@<node-ip>
```

Only add the `LD_PRELOAD` shim if the plain command above still reports `No user exists for uid <uid>`.

## Fix `~/.kube/config` without `minikube update-context`

```
kubectl config set-cluster minikube --server=https://<node-ip>:8443 \
  --certificate-authority=~/.minikube/ca.crt --embed-certs=true
kubectl config set-credentials minikube-user \
  --client-certificate=~/.minikube/profiles/minikube/client.crt \
  --client-key=~/.minikube/profiles/minikube/client.key --embed-certs=true
kubectl config set-context hippocampus --cluster=minikube --user=minikube-user --namespace=default
kubectl config use-context hippocampus
```

The `client.crt` subject carries `O=system:masters`, so it authenticates as cluster-admin without needing RBAC to already be bootstrapped — the only credential that works before `poststarthook/rbac/bootstrap-roles` has run.
