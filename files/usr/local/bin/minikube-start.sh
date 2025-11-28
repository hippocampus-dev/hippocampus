#!/usr/bin/env -S bash -l

set -e

if [ ! -f ~/.minikube/x86_64.iso ]; then
  d=$(mktemp -d)
  git clone https://github.com/kubernetes/minikube.git -b $(minikube version | head -n1 | awk '{print $NF}') --single-branch --depth 1 ${d}
  cd ${d}

  if [ "${MINIKUBE_KERNEL_VERSION}" == "6.1" ]; then
    sed -ri 's/KERNEL_VERSION \?= (.+)/KERNEL_VERSION ?= 6.1.108/g' Makefile
    sed -ri 's/BR2_LINUX_KERNEL_CUSTOM_VERSION_VALUE="(.+)"/BR2_LINUX_KERNEL_CUSTOM_VERSION_VALUE="6.1.108"/g' deploy/iso/minikube-iso/configs/minikube_x86_64_defconfig
    sed -ri 's/BR2_PACKAGE_HOST_LINUX_HEADERS_CUSTOM_(.+)=y/BR2_PACKAGE_HOST_LINUX_HEADERS_CUSTOM_6_1=y/g' deploy/iso/minikube-iso/configs/minikube_x86_64_defconfig

    sed -ri 's|HYPERV_DAEMONS_SITE = https://www.kernel.org/pub/linux/kernel/v[0-9].x|HYPERV_DAEMONS_SITE = https://www.kernel.org/pub/linux/kernel/v6.x|g' deploy/iso/minikube-iso/arch/x86_64/package/hyperv-daemons/hyperv-daemons.mk
  elif [ "${MINIKUBE_KERNEL_VERSION}" == "6.6" ]; then
    sed -ri 's/KERNEL_VERSION \?= (.+)/KERNEL_VERSION ?= 6.6.49/g' Makefile
    sed -ri 's/BR2_LINUX_KERNEL_CUSTOM_VERSION_VALUE="(.+)"/BR2_LINUX_KERNEL_CUSTOM_VERSION_VALUE="6.6.49"/g' deploy/iso/minikube-iso/configs/minikube_x86_64_defconfig
    sed -ri 's/BR2_PACKAGE_HOST_LINUX_HEADERS_CUSTOM_(.+)=y/BR2_PACKAGE_HOST_LINUX_HEADERS_CUSTOM_6_6=y/g' deploy/iso/minikube-iso/configs/minikube_x86_64_defconfig

    sed -ri 's|HYPERV_DAEMONS_SITE = https://www.kernel.org/pub/linux/kernel/v[0-9].x|HYPERV_DAEMONS_SITE = https://www.kernel.org/pub/linux/kernel/v6.x|g' deploy/iso/minikube-iso/arch/x86_64/package/hyperv-daemons/hyperv-daemons.mk

    # https://github.com/torvalds/linux/commit/82b0945ce2c2d636d5e893ad50210875c929f257
    sed -ri 's/hv_fcopy_daemon( \\\)?$/hv_fcopy_uio_daemon\1/g' deploy/iso/minikube-iso/arch/x86_64/package/hyperv-daemons/hyperv-daemons.mk
  else
    sed -ri 's/KERNEL_VERSION \?= (.+)/KERNEL_VERSION ?= 5.15.166/g' Makefile
    sed -ri 's/BR2_LINUX_KERNEL_CUSTOM_VERSION_VALUE="(.+)"/BR2_LINUX_KERNEL_CUSTOM_VERSION_VALUE="5.15.166"/g' deploy/iso/minikube-iso/configs/minikube_x86_64_defconfig
    sed -ri 's/BR2_PACKAGE_HOST_LINUX_HEADERS_CUSTOM_(.+)=y/BR2_PACKAGE_HOST_LINUX_HEADERS_CUSTOM_5_15=y/g' deploy/iso/minikube-iso/configs/minikube_x86_64_defconfig
  fi

  # https://github.com/torvalds/linux/blob/v6.10/kernel/trace/Kconfig#L766
  echo "CONFIG_FUNCTION_ERROR_INJECTION=y" >> deploy/iso/minikube-iso/board/minikube/x86_64/linux_x86_64_defconfig
  echo "CONFIG_BPF_KPROBE_OVERRIDE=y" >> deploy/iso/minikube-iso/board/minikube/x86_64/linux_x86_64_defconfig

  make buildroot-image
  make out/minikube-x86_64.iso
  mv out/minikube-amd64.iso ~/.minikube/x86_64.iso
fi

mkdir -p ~/.minikube/files/var/lib/kubelet
[ -f ~/.docker/config.json ] && cp ~/.docker/config.json ~/.minikube/files/var/lib/kubelet/config.json

mkdir -p ~/.minikube/files/etc/ssl/certs
if [ ! -f ~/.minikube/files/etc/ssl/certs/encryption.yaml ]; then
  cat <<EOS | sudo tee ~/.minikube/files/etc/ssl/certs/encryption.yaml > /dev/null
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

if [ ! -f ~/.minikube/files/etc/ssl/certs/audit-policy.yaml ]; then
  cat <<EOS | sudo tee ~/.minikube/files/etc/ssl/certs/audit-policy.yaml > /dev/null
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
EOS
fi

if [ ! -f /var/lib/libvirt/network/mk-minikube.xml ]; then
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

RUNTIME_OPTION="--kubernetes-version=v1.32.0 --container-runtime=containerd"
CNI_OPTION="--cni=false --force --extra-config=kubeadm.skip-phases=addon/kube-proxy"
FEATURE_GATE_OPTION="--feature-gates=HPAScaleToZero=true,MutatingAdmissionPolicy=true,InPlacePodVerticalScaling=true,ResourceHealthStatus=true,JobSuccessPolicy=true --extra-config=apiserver.runtime-config=admissionregistration.k8s.io/v1alpha1=true"
AUDIT_OPTION="--extra-config=apiserver.audit-policy-file=/etc/ssl/certs/audit-policy.yaml --extra-config=apiserver.audit-log-path=-"
EVICTION_OPTION="--extra-config=kubelet.eviction-soft='memory.available<10%' --extra-config=kubelet.eviction-soft-grace-period='memory.available=1m' --extra-config=kubelet.eviction-hard='memory.available<5%'"
IMAGE_PULL_OPTION="--extra-config=kubelet.serialize-image-pulls=false"
# NOTE: Pay attention to /proc/sys/kernel/pid_max when changing kubelet.max-pods
RESOURCE_OPTION="--extra-config=kubelet.max-pods=300 --cpus=16 --memory=56g --disk-size=200g --extra-config=kubelet.system-reserved=cpu=500m,memory=1Gi,ephemeral-storage=10Gi,pid=1000 --extra-config=kubelet.containerLogMaxSize=5Mi --extra-config=kubelet.containerLogMaxFiles=1"
ENCRYPTION_OPTION="--extra-config=apiserver.encryption-provider-config=/etc/ssl/certs/encryption.yaml"
APF_OPTIONS="--extra-config=apiserver.max-mutating-requests-inflight=200 --extra-config=apiserver.max-requests-inflight=400"
DISABLE_LEADER_ELECT_OPTION="--extra-config=controller-manager.leader-elect=false --extra-config=scheduler.leader-elect=false"
IMAGE_GC_OPTION="--extra-config=kubelet.image-gc-high-threshold=80 --extra-config=kubelet.image-gc-low-threshold=70"
#ETCD_SCRAPE_OPTION="--extra-config=etcd.listen-metrics-urls=http://0.0.0.0:2381"
SCRAPE_OPTION="--extra-config=controller-manager.bind-address=0.0.0.0 --extra-config=scheduler.bind-address=0.0.0.0 $ETCD_SCRAPE_OPTION"

OPTION="--vm-driver=kvm2 ${RUNTIME_OPTION} ${CNI_OPTION} ${FEATURE_GATE_OPTION} ${AUDIT_OPTION} ${EVICTION_OPTION} ${IMAGE_PULL_OPTION} ${RESOURCE_OPTION} ${ENCRYPTION_OPTION} ${APF_OPTIONS} ${DISABLE_LEADER_ELECT_OPTION} ${IMAGE_GC_OPTION} ${SCRAPE_OPTION} --extra-config=kubelet.housekeeping-interval=15s --iso-url=file:///home/${USER}/.minikube/x86_64.iso"

TMPDIR=/var/tmp minikube start ${OPTION}

# https://docs.cilium.io/en/stable/installation/taints/
kubectl taint nodes minikube node.cilium.io/agent-not-ready=true:NoExecute --overwrite

if kubectl get ValidatingAdmissionPolicy temporary.validating.kaidotio.github.io 2> /dev/null; then
  kubectl delete ValidatingAdmissionPolicy temporary.validating.kaidotio.github.io
fi
if kubectl get ValidatingAdmissionPolicyBinding temporary.validating.kaidotio.github.io 2> /dev/null; then
  kubectl delete ValidatingAdmissionPolicyBinding temporary.validating.kaidotio.github.io
fi
