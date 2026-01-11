import collections.abc
import os

import jupyterhub.utils
import kubespawner
import tornado.httpclient
import traitlets.config

c = get_config()  # noqa

c.JupyterHub: jupyterhub.app.JupyterHub = c.JupyterHub  # noqa
c.Authenticator: jupyterhub.auth.Authenticator = c.Authenticator  # noqa
c.Spawner: jupyterhub.spawner.Spawner = c.Spawner  # noqa
c.KubeSpawner: kubespawner.KubeSpawner = c.KubeSpawner  # noqa
c.ConfigurableHTTPProxy: jupyterhub.proxy.ConfigurableHTTPProxy = c.ConfigurableHTTPProxy  # noqa

tornado.httpclient.AsyncHTTPClient.configure("tornado.curl_httpclient.CurlAsyncHTTPClient")
c.JupyterHub.tornado_settings = {
    "slow_spawn_timeout": 0,
    "websocket_max_message_size": 1 * 1024 * 1024 * 1024,  # 1GB
}


class HeaderAuthenticator(jupyterhub.auth.Authenticator):
    auto_login = True

    @tornado.gen.coroutine
    def authenticate(self, handler: tornado.web.RequestHandler, data: collections.abc.Mapping):
        return handler.request.headers.get("X-Auth-Request-User")

    def get_handlers(self, app):
        return []


c.JupyterHub.admin_access = False
c.Authenticator.allow_all = True
c.Authenticator.admin_users = {"kaidotio"}
c.JupyterHub.authenticator_class = HeaderAuthenticator


def _strtobool(val):
    if isinstance(val, str):
        return val.lower() in {"1", "true"}
    return bool(val)


class DynamicKubeSpawner(kubespawner.KubeSpawner):
    def start(self):
        user = self.user.name
        is_admin = user in c.Authenticator.admin_users

        params = {
            "user": user,
            "is_admin": is_admin,
            "is_not_admin": not is_admin,
        }

        for init_container in self.init_containers:
            for volume_mount in init_container.get("volumeMounts", []):
                if "mountPath" in volume_mount:
                    volume_mount["mountPath"] = volume_mount["mountPath"].format(**params)
                if "subPath" in volume_mount:
                    volume_mount["subPath"] = volume_mount["subPath"].format(**params)
                if "readOnly" in volume_mount:
                    volume_mount["readOnly"] = _strtobool(volume_mount["readOnly"].format(**params))

        for volume in self.volumes:
            if "hostPath" in volume and "path" in volume["hostPath"]:
                volume["hostPath"]["path"] = volume["hostPath"]["path"].format(**params)
            if "nfs" in volume and "path" in volume["nfs"]:
                volume["nfs"]["path"] = volume["nfs"]["path"].format(**params)

        for volume_mount in self.volume_mounts:
            if "mountPath" in volume_mount:
                volume_mount["mountPath"] = volume_mount["mountPath"].format(**params)
            if "subPath" in volume_mount:
                volume_mount["subPath"] = volume_mount["subPath"].format(**params)
            if "readOnly" in volume_mount:
                volume_mount["readOnly"] = _strtobool(volume_mount["readOnly"].format(**params))

        return super().start()


c.JupyterHub.spawner_class = DynamicKubeSpawner
c.KubeSpawner.profile_list = [
    {
        "display_name": "Standard Notebook",
        "description": "4 CPU / 32 GB RAM",
        "default": True,
    },
    {
        "display_name": "GPU Notebook",
        "description": "4 CPU / 32 GB RAM / 1 GPU",
        "kubespawner_override": {
            "extra_pod_config": {
                "runtimeClassName": "nvidia",
            },
            "extra_resource_guarantees": {"nvidia.com/gpu": "1"},
            "extra_resource_limits": {"nvidia.com/gpu": "1"},
        },
    },
]
c.KubeSpawner.start_timeout = 300
c.KubeSpawner.service_account = "jupyterhub-singleuser-server"
c.KubeSpawner.automount_service_account_token = True
c.KubeSpawner.image = "ghcr.io/hippocampus-dev/hippocampus/singleuser-notebook:main"
c.KubeSpawner.image_pull_policy = "Always"
c.KubeSpawner.environment = {
    "NOTEBOOK_ARGS": "--FileCheckpoints.checkpoint_dir=/mnt/.ipynb_checkpoints",
}
c.KubeSpawner.cpu_guarantee = 1
c.KubeSpawner.cpu_limit = 4
c.KubeSpawner.mem_guarantee = "256M"
c.KubeSpawner.mem_limit = "32G"
c.KubeSpawner.extra_resource_guarantees = {"ephemeral-storage": "1G"}
c.KubeSpawner.namespace = os.environ.get("POD_NAMESPACE", "jupyterhub")
c.KubeSpawner.priority_class_name = "low"
c.KubeSpawner.init_containers = [
    {
        "name": "chown",
        "image": "ghcr.io/hippocampus-dev/hippocampus/ephemeral-container:main",
        "command": ["sudo", "chown", "1000:1000", "/mnt/persistent"],
        "resources": {
            "requests": {
                "cpu": "10m",
                "memory": "32Mi",
            },
            "limits": {
                "cpu": "10m",
                "memory": "32Mi",
            },
        },
        # for hostPath
        "securityContext": {
            "privileged": True,
        },
        "volumeMounts": [
            {
                "name": "persistent",
                "mountPath": "/mnt/persistent",
                "subPath": "{user}",
            },
        ],
    },
]
c.KubeSpawner.volumes = [
    {
        "name": "persistent",
        "hostPath": {
            "path": "/data/jupyterhub/persistent",
        },
    },
    {
        "name": "shared",
        "hostPath": {
            "path": "/data/jupyterhub/shared",
        },
    },
    {
        "name": "torch",
        "hostPath": {
            "path": "/data/jupyterhub/.cache/torch",
        },
    },
    {
        "name": "checkpoints",
        "emptyDir": {
            "medium": "Memory",
        },
    },
]
c.KubeSpawner.volume_mounts = [
    {
        "name": "persistent",
        "mountPath": "/home/jovyan/persistent",
        "subPath": "{user}",
    },
    {
        "name": "shared",
        "mountPath": "/home/jovyan/shared",
        "readOnly": "{is_not_admin}",
    },
    {
        "name": "torch",
        "mountPath": "/home/jovyan/.cache/torch",
        "readOnly": "{is_not_admin}",
    },
    {
        "name": "checkpoints",
        "mountPath": "/mnt/.ipynb_checkpoints",
    },
]
c.KubeSpawner.extra_labels = {
    "app.kubernetes.io/name": "jupyterhub",
    "app.kubernetes.io/component": "singleuser-server",
    "sidecar.istio.io/inject": "true",
    # Istio Telemetry and AuthorizationPolicy with istio-egressgateway cannot log https traffic
    "policy.cilium.io/l7-visibility": "true",
    # Enable Hubble Exporter
    "hubble.cilium.io/export.source": "true",
}
c.KubeSpawner.extra_annotations = {
    "sidecar.istio.io/proxyCPU": "10m",
    "sidecar.istio.io/proxyMemory": "128Mi",
    "sidecar.istio.io/proxyCPULimit": "100m",
    "sidecar.istio.io/proxyMemoryLimit": "128Mi",
    "prometheus.io/scrape": "true",
    "prometheus.io/scheme": "http",
    "prometheus.io/port": "15020",
    "prometheus.io/path": "/stats/prometheus",
}
c.KubeSpawner.services_enabled = True

c.JupyterHub.allow_named_servers = True
c.JupyterHub.named_server_limit_per_user = 3
c.JupyterHub.concurrent_spawn_limit = 0

# https://jupyterhub.readthedocs.io/en/stable/howto/separate-proxy.html#running-proxy-separately-from-the-hub
c.JupyterHub.cleanup_servers = False
c.ConfigurableHTTPProxy.should_start = False
c.ConfigurableHTTPProxy.auth_token = os.environ.get("CONFIGPROXY_AUTH_TOKEN", "")
c.ConfigurableHTTPProxy.api_url = "http://jupyterhub-proxy.jupyterhub.svc.cluster.local:8000"
c.ConfigurableHTTPProxy.log_level = "warn"

hub_container_port = 8081
c.JupyterHub.hub_bind_url = f"http://0.0.0.0:{hub_container_port}"
c.JupyterHub.hub_connect_url = f"http://jupyterhub-hub.jupyterhub.svc.cluster.local:{hub_container_port}"

c.JupyterHub.authenticate_prometheus = False

# Check that the proxy has routes appropriately setup
c.JupyterHub.last_activity_interval = 60

base_url: traitlets.config.LazyConfigValue = c.JupyterHub.base_url
c.JupyterHub.services = [
    {
        "name": "jupyterhub-idle-culler",
        "command": [
            "python",
            "-m",
            "jupyterhub_idle_culler",
            f"--url=http://127.0.0.1:{hub_container_port}/{jupyterhub.utils.url_path_join(base_url.get_value(''), 'hub/api')}",
            "--timeout=3600",
            "--cull-every=600",
            "--concurrency=10",
            "--max-age=0",
        ],
    }
]
c.JupyterHub.load_roles = [
    {
        "name": "jupyterhub-idle-culler",
        "scopes": [
            "list:users",
            "read:users:activity",
            "read:servers",
            "delete:servers",
        ],
        "services": ["jupyterhub-idle-culler"],
    }
]

if os.environ.get("LOG_LEVEL") == "DEBUG":
    c.JupyterHub.log_level = "DEBUG"
    c.Spawner.debug = True
