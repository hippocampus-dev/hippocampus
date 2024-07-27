#!/usr/bin/env bash

set -eo pipefail

mkdir -p /home/kai/.minikube/files/var/lib/kubelet
[ -e /home/kai/.docker/config.json ] && cp /home/kai/.docker/config.json /home/kai/.minikube/files/var/lib/kubelet/config.json

mkdir -p /home/kai/.minikube/files/etc/ssl/certs
if [ ! -e /home/kai/.minikube/files/etc/ssl/certs/encryption.yaml ]; then
  cat <<EOS | sudo tee /home/kai/.minikube/files/etc/ssl/certs/encryption.yaml > /dev/null
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - secretbox:
          keys:
            - name: key
              secret: $(head -c 32 /dev/urandom | base64 -w0)
      - identity: {}
EOS
fi

if [ ! -e /home/kai/.minikube/files/etc/ssl/certs/audit-policy.yaml ]; then
  cat <<EOS | sudo tee /home/kai/.minikube/files/etc/ssl/certs/audit-policy.yaml > /dev/null
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
EOS
fi

if [ ! -e /var/lib/libvirt/network/mk-minikube.xml ]; then
  cat <<EOS | sudo tee /var/lib/libvirt/network/mk-minikube.xml > /dev/null
<network connections='1'>
  <name>mk-minikube</name>
  <uuid>77e603d8-7747-451e-8391-13bb31022974</uuid>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='virbr1' stp='on' delay='0'/>
  <mac address='52:54:00:21:3e:cb'/>
  <dns enable='no'/>
  <ip address='192.168.39.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.39.2' end='192.168.39.253'/>
    </dhcp>
  </ip>
</network>
EOS
fi
sudo virsh net-define /var/lib/libvirt/network/mk-minikube.xml

RUNTIME_OPTION="--kubernetes-version=v1.29.2 --container-runtime=containerd"
CNI_OPTION="--cni=false --force --extra-config=kubeadm.skip-phases=addon/kube-proxy"
FEATURE_GATE_OPTION="--feature-gates=HPAScaleToZero=true,ValidatingAdmissionPolicy=true --extra-config=apiserver.runtime-config=admissionregistration.k8s.io/v1beta1"
# Reduce disk I/O
#AUDIT_OPTION="--extra-config=apiserver.audit-policy-file=/etc/ssl/certs/audit-policy.yaml --extra-config=apiserver.audit-log-path=-"
EVICTION_OPTION="--extra-config=kubelet.eviction-soft='memory.available<10%' --extra-config=kubelet.eviction-soft-grace-period='memory.available=1m' --extra-config=kubelet.eviction-hard='memory.available<5%'"
# NOTE: Pay attention to /proc/sys/kernel/pid_max when changing kubelet.max-pods
RESOURCE_OPTION="--extra-config=kubelet.max-pods=300 --cpus=16 --memory=64g --disk-size=200g --extra-config=kubelet.system-reserved=cpu=500m,memory=1Gi,ephemeral-storage=10Gi,pid=1000"
ENCRYPTION_OPTION="--extra-config=apiserver.encryption-provider-config=/etc/ssl/certs/encryption.yaml"
APF_OPTIONS="--extra-config=apiserver.max-mutating-requests-inflight=200 --extra-config=apiserver.max-requests-inflight=400"
DISABLE_LEADER_ELECT_OPTION="--extra-config=controller-manager.leader-elect=false --extra-config=scheduler.leader-elect=false"

OPTION="--vm-driver=kvm2 $RUNTIME_OPTION $CNI_OPTION $FEATURE_GATE_OPTION $AUDIT_OPTION $EVICTION_OPTION $RESOURCE_OPTION $ENCRYPTION_OPTION $APF_OPTIONS $DISABLE_LEADER_ELECT_OPTION --extra-config=controller-manager.bind-address=0.0.0.0 --extra-config=scheduler.bind-address=0.0.0.0 --extra-config=etcd.listen-metrics-urls=http://0.0.0.0:2381 --extra-config=kubelet.housekeeping-interval=15s"

minikube start $OPTION
