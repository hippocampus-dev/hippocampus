# relocatable-volume-controller

<!-- TOC -->
* [relocatable-volume-controller](#relocatable-volume-controller)
  * [Usage](#usage)
    * [Relocation](#relocation)
    * [Discard](#discard)
  * [Development](#development)
<!-- TOC -->

relocatable-volume-controller is a Kubernetes Custom Controller and admission webhook that moves the node-local PersistentVolumes of a pod to another node on request, and discards them when no schedulable node can reach them, so the pod's owner recreates both there.

## Usage

Annotate a pod template with the comma-separated names of the volumes that follow the pod, whose contents may be lost when their node leaves first, and set `method` to `transfer` to copy them on a relocation instead of discarding them, which is the default.

```yaml
relocatable-volume-controller.kaidotio.github.io/volumes: data
relocatable-volume-controller.kaidotio.github.io/method: transfer
```

See `examples/`.

### Relocation

Annotate the node holding the volumes with the name of the node to move them to, and wait until the controller removes the annotation.
A PersistentVolume carrying no `spec.nodeAffinity.required` counts as being on the node of its pod under `method: transfer` and is left in place otherwise.

```sh
$ kubectl annotate node source relocatable-volume-controller.kaidotio.github.io/relocate-to=target
```

The controller handles one pod at a time while the target is `Ready` and uncordoned.
A pod without `method: transfer` loses its PersistentVolumeClaims and is evicted, so its owner recreates both wherever the scheduler picks.
A pod with it moves in these steps, and the controller waits for it to become `Ready` on the target before the next pod.

| Step | What happens |
|------|--------------|
| Stop | Marks the PersistentVolumeClaims and evicts the pod, whose replacement the webhook holds with a scheduling gate |
| Copy | Runs a sender on the source and a receiver on the target under the security context of the pod, streaming the volume into a new PersistentVolume of the default StorageClass |
| Switch | Reserves the new PersistentVolume for the claim name, deletes the old claim and creates it again on the new volume |
| Start | Removes the scheduling gate |

A failed copy starts over, and a finished one holds only the regular files, directories and symlinks of the volume, all owned by the user running the receiver.
Removing the annotation gives up on every volume not yet switched, and the pod returns to its old volume.
The controller does the same while the source is not `Ready`, and starts over once it returns.

### Discard

The controller force deletes a terminating pod once its deletion grace period has passed while its node is not `Ready`, so its owner can recreate it.
Otherwise the controller acts only on a pod that is not bound to a node and whose `PodScheduled` condition is `False` with reason `Unschedulable`, and handles each named volume by the first row that matches.

| Named PersistentVolumeClaim | Action |
|-----------------------------|--------|
| Being relocated | Leaves the volume alone |
| Missing or being deleted | Deletes the pod |
| Bound to a PersistentVolume carrying `spec.nodeAffinity.required` that no `Ready` node matches | Deletes the PersistentVolumeClaim and the pod |
| Anything else | Leaves the pod alone |

## Development

```sh
$ make dev
```
