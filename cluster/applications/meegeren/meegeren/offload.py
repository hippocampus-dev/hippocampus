import collections.abc
import contextlib

import torch


@contextlib.contextmanager
def placed(
    device: str, keep_on_device: bool, *modules: torch.nn.Module
) -> collections.abc.Iterator[None]:
    try:
        if not keep_on_device:
            for module in modules:
                module.to(device)
        yield
    finally:
        if not keep_on_device:
            for module in modules:
                module.to("cpu")
