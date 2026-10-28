resource "google_project_service" "storage" {
  service            = "storage.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iam_credentials" {
  service            = "iamcredentials.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "drive" {
  service            = "drive.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloud_resource_manager" {
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloud_billing" {
  service            = "cloudbilling.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iam" {
  service            = "iam.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "service_usage" {
  service            = "serviceusage.googleapis.com"
  disable_on_destroy = false
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github"
  display_name              = "GitHub Actions"

  depends_on = [google_project_service.iam_credentials]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "GitHub Actions OIDC"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }

  attribute_condition = "attribute.repository == \"hippocampus-dev/hippocampus\" && attribute.ref == \"refs/heads/main\""

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account" "drive_sync" {
  account_id   = "drive-sync"
  display_name = "Google Drive Sync"

  depends_on = [google_project_service.iam_credentials, google_project_service.drive]
}

resource "google_service_account_iam_member" "drive_sync_workload_identity" {
  service_account_id = google_service_account.drive_sync.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/hippocampus-dev/hippocampus"
}

resource "google_storage_bucket" "sync" {
  name     = "kaidotio-sync"
  location = "ASIA-NORTHEAST1"

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      num_newer_versions = 3
      with_state         = "ARCHIVED"
    }
    action {
      type = "Delete"
    }
  }

  lifecycle_rule {
    condition {
      days_since_noncurrent_time = 7
      with_state                 = "ANY"
    }
    action {
      type = "Delete"
    }
  }

  depends_on = [google_project_service.storage]
}

resource "google_project_service" "compute" {
  service            = "compute.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iap" {
  service            = "iap.googleapis.com"
  disable_on_destroy = false
}

data "google_project" "current" {}

resource "google_compute_network" "minikube_gce_worker" {
  name                    = "minikube-gce-worker"
  auto_create_subnetworks = false

  depends_on = [google_project_service.compute]
}

resource "google_compute_subnetwork" "minikube_gce_worker" {
  name    = "minikube-gce-worker"
  region  = "asia-northeast1"
  network = google_compute_network.minikube_gce_worker.id
  # Kept out of 10.0.0.0/8, the default Cilium cluster-pool PodCIDR: https://github.com/cilium/cilium/blob/v1.16.19/install/kubernetes/cilium/values.yaml
  ip_cidr_range = "192.168.40.0/24"
}

resource "google_compute_firewall" "minikube_gce_worker_wireguard" {
  name    = "minikube-gce-worker-wireguard"
  network = google_compute_network.minikube_gce_worker.id
  # The IPv4 the Arch Linux host reaches GCE from, which NURO光 assigns dynamically
  source_ranges = ["219.104.132.49/32"]

  allow {
    protocol = "udp"
    ports    = ["51820"]
  }
}

resource "google_compute_firewall" "minikube_gce_worker_iap" {
  name    = "minikube-gce-worker-iap"
  network = google_compute_network.minikube_gce_worker.id
  # https://cloud.google.com/iap/docs/using-tcp-forwarding#create-firewall-rule
  source_ranges = ["35.235.240.0/20"]

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_firewall" "minikube_gce_worker_internal" {
  name          = "minikube-gce-worker-internal"
  network       = google_compute_network.minikube_gce_worker.id
  source_ranges = [google_compute_subnetwork.minikube_gce_worker.ip_cidr_range]

  allow {
    protocol = "all"
  }
}

data "external" "minikube" {
  program = ["bash", "${path.module}/minikube-versions.sh"]
}

locals {
  # noble's linux-image-gcp moved to 7.0 on 2026-09-03: https://launchpad.net/ubuntu/noble/amd64/linux-image-gcp
  # nvidia-dkms-open 615.71.10 fails to build on 7.0.0-1014-gcp: https://github.com/NVIDIA/open-gpu-kernel-modules/issues/1465
  minikube_gce_worker_source_image = "projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260826"

  minikube_gce_worker_image_metadata = { for name, nvidia in { "minikube-gce-worker" = false, "minikube-gce-worker-gpu" = true } : name => {
    startup-script     = file("${path.module}/minikube-gce-worker-image.sh")
    kubernetes-version = data.external.minikube.result.kubernetes_version
    containerd-version = data.external.minikube.result.containerd_version
    runc-version       = data.external.minikube.result.runc_version
    nvidia             = tostring(nvidia)
    images             = join(" ", nvidia ? [data.external.minikube.result.cilium_image, data.external.minikube.result.nvidia_device_plugin_image] : [data.external.minikube.result.cilium_image])
  } }
}

resource "google_compute_instance" "minikube_gce_worker_image_builder" {
  for_each = local.minikube_gce_worker_image_metadata
  # files/usr/local/bin/minikube-gce-worker.sh joins every instance named minikube-gce-worker-*
  name         = "builder-${each.key}-${substr(sha1(jsonencode([local.minikube_gce_worker_source_image, each.value])), 0, 8)}"
  machine_type = "e2-standard-4"
  zone         = "asia-northeast1-a"

  boot_disk {
    initialize_params {
      image = local.minikube_gce_worker_source_image
      type  = "pd-standard"
      size  = 10
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.minikube_gce_worker.id

    access_config {}
  }

  metadata = each.value
}

# An image can be taken from a boot disk without forcing it only once its instance is TERMINATED: https://docs.cloud.google.com/compute/docs/images/create-custom
resource "terraform_data" "minikube_gce_worker_image_builder" {
  for_each         = google_compute_instance.minikube_gce_worker_image_builder
  triggers_replace = each.value.instance_id

  provisioner "local-exec" {
    interpreter = ["bash"]
    command     = "${path.module}/wait-minikube-gce-worker-image-builder.sh"
    environment = {
      INSTANCE = each.value.name
      ZONE     = each.value.zone
      PROJECT  = each.value.project
    }
  }
}

resource "google_compute_image" "minikube_gce_worker" {
  for_each = terraform_data.minikube_gce_worker_image_builder
  # create_before_destroy on the instance templates also applies here: https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle
  name        = "${each.key}-${google_compute_instance.minikube_gce_worker_image_builder[each.key].instance_id}"
  source_disk = google_compute_instance.minikube_gce_worker_image_builder[each.key].boot_disk[0].source
}

resource "google_compute_instance_template" "minikube_gce_worker" {
  name_prefix  = "minikube-gce-worker-"
  machine_type = "e2-standard-4"
  region       = "asia-northeast1"

  disk {
    source_image = google_compute_image.minikube_gce_worker["minikube-gce-worker"].self_link
    disk_type    = "pd-balanced"
    disk_size_gb = 50
    boot         = true
    auto_delete  = true
  }

  network_interface {
    subnetwork = google_compute_subnetwork.minikube_gce_worker.id

    access_config {}
  }

  # A MIG rejects instance_termination_action DELETE with "Spot virtual machines with termination action set to DELETE cannot be used with Managed Instance Groups."
  scheduling {
    provisioning_model          = "SPOT"
    preemptible                 = true
    automatic_restart           = false
    instance_termination_action = "STOP"
  }

  metadata = {
    # cluster-autoscaler builds the node it simulates for an empty group from this: https://github.com/kubernetes/autoscaler/blob/482f759b8/cluster-autoscaler/cloudprovider/gce/templates.go
    kube-env = "AUTOSCALER_ENV_VARS: node_taints=node.kaidotio.github.io/remote-worker=gce:NoSchedule;os=linux;arch=amd64"
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "google_compute_instance_group_manager" "minikube_gce_worker" {
  name               = "minikube-gce-worker"
  zone               = "asia-northeast1-a"
  base_instance_name = "minikube-gce-worker"

  version {
    instance_template = google_compute_instance_template.minikube_gce_worker.self_link_unique
  }

  # A MIG repairs a preempted Spot VM by recreating it: https://docs.cloud.google.com/compute/docs/instance-groups/about-repair
  # A recreate keeps the instance name and may keep its creation timestamp: https://docs.cloud.google.com/compute/docs/instance-groups/working-with-managed-instances
  # files/usr/local/bin/minikube-gce-worker.sh tells a recreated instance apart only by its creation timestamp
  instance_lifecycle_policy {
    default_action_on_failure = "DO_NOTHING"
  }

  # cluster-autoscaler owns the size
  lifecycle {
    ignore_changes = [target_size]
  }
}

resource "google_compute_instance_template" "minikube_gce_worker_gpu" {
  name_prefix  = "minikube-gce-worker-gpu-"
  machine_type = "n1-standard-4"
  region       = "asia-northeast1"

  disk {
    source_image = google_compute_image.minikube_gce_worker["minikube-gce-worker-gpu"].self_link
    disk_type    = "pd-balanced"
    disk_size_gb = 50
    boot         = true
    auto_delete  = true
  }

  guest_accelerator {
    type  = "nvidia-tesla-t4"
    count = 1
  }

  network_interface {
    subnetwork = google_compute_subnetwork.minikube_gce_worker.id

    access_config {}
  }

  # A MIG rejects instance_termination_action DELETE with "Spot virtual machines with termination action set to DELETE cannot be used with Managed Instance Groups."
  scheduling {
    provisioning_model          = "SPOT"
    preemptible                 = true
    automatic_restart           = false
    instance_termination_action = "STOP"
  }

  metadata = {
    # cluster-autoscaler builds the node it simulates for an empty group from this: https://github.com/kubernetes/autoscaler/blob/482f759b8/cluster-autoscaler/cloudprovider/gce/templates.go
    # cluster-autoscaler tells GPU nodes apart by this label: https://github.com/kubernetes/autoscaler/blob/482f759b8/cluster-autoscaler/cloudprovider/gce/gce_cloud_provider.go#L53
    kube-env = "AUTOSCALER_ENV_VARS: node_labels=cloud.google.com/gke-accelerator=nvidia-tesla-t4;node_taints=node.kaidotio.github.io/remote-worker=gce:NoSchedule,nvidia.com/gpu=:NoSchedule;os=linux;arch=amd64"
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "google_compute_instance_group_manager" "minikube_gce_worker_gpu" {
  name               = "minikube-gce-worker-gpu"
  zone               = "asia-northeast1-a"
  base_instance_name = "minikube-gce-worker-gpu"

  version {
    instance_template = google_compute_instance_template.minikube_gce_worker_gpu.self_link_unique
  }

  # A MIG repairs a preempted Spot VM by recreating it: https://docs.cloud.google.com/compute/docs/instance-groups/about-repair
  # A recreate keeps the instance name and may keep its creation timestamp: https://docs.cloud.google.com/compute/docs/instance-groups/working-with-managed-instances
  # files/usr/local/bin/minikube-gce-worker.sh tells a recreated instance apart only by its creation timestamp
  instance_lifecycle_policy {
    default_action_on_failure = "DO_NOTHING"
  }

  # cluster-autoscaler owns the size
  lifecycle {
    ignore_changes = [target_size]
  }
}

resource "google_project_service" "cloud_quotas" {
  service            = "cloudquotas.googleapis.com"
  disable_on_destroy = false
}

resource "google_cloud_quotas_quota_preference" "gpus_all_regions" {
  parent        = "projects/${data.google_project.current.project_id}"
  service       = "compute.googleapis.com"
  quota_id      = "GPUS-ALL-REGIONS-per-project"
  contact_email = "kaidotio@gmail.com"

  quota_config {
    preferred_value = 1
  }

  depends_on = [google_project_service.cloud_quotas]
}

# Spot VMs consume standard quota until preemptible quota is ever requested, after which they cannot revert: https://docs.cloud.google.com/compute/resource-usage
resource "google_cloud_quotas_quota_preference" "nvidia_t4_gpus" {
  parent        = "projects/${data.google_project.current.project_id}"
  service       = "compute.googleapis.com"
  quota_id      = "NVIDIA-T4-GPUS-per-project-region"
  dimensions    = { region = "asia-northeast1" }
  contact_email = "kaidotio@gmail.com"

  quota_config {
    preferred_value = 1
  }

  depends_on = [google_project_service.cloud_quotas]
}

resource "google_service_account" "cluster_autoscaler" {
  account_id   = "cluster-autoscaler"
  display_name = "Cluster Autoscaler"

  depends_on = [google_project_service.iam_credentials]
}

resource "google_project_iam_member" "cluster_autoscaler" {
  project = data.google_project.current.project_id
  role    = "roles/compute.instanceAdmin.v1"
  member  = "serviceAccount:${google_service_account.cluster_autoscaler.email}"
}

resource "google_service_account_key" "cluster_autoscaler" {
  service_account_id = google_service_account.cluster_autoscaler.name
}

resource "google_project_service" "billing_budgets" {
  service            = "billingbudgets.googleapis.com"
  disable_on_destroy = false
}

resource "google_billing_budget" "monthly" {
  billing_account = data.google_project.current.billing_account
  display_name    = "monthly"

  budget_filter {
    projects = ["projects/${data.google_project.current.number}"]
  }

  amount {
    specified_amount {
      currency_code = "JPY"
      units         = "3000"
    }
  }

  threshold_rules {
    threshold_percent = 0.5
  }

  threshold_rules {
    threshold_percent = 0.9
  }

  threshold_rules {
    threshold_percent = 1.0
  }

  threshold_rules {
    threshold_percent = 1.0
    spend_basis       = "FORECASTED_SPEND"
  }

  depends_on = [google_project_service.billing_budgets]
}
