import collections.abc
import contextlib
import threading

import torch


class Interrupted(Exception):
    pass


@contextlib.contextmanager
def interruptible(
    cancelled: threading.Event, *modules: torch.nn.Module
) -> collections.abc.Iterator[None]:
    def interrupt(module: torch.nn.Module, args: tuple) -> None:
        if cancelled.is_set():
            raise Interrupted

    handles = [
        submodule.register_forward_pre_hook(interrupt)
        for module in modules
        for submodule in module.modules()
    ]
    try:
        yield
    except Interrupted:
        pass
    finally:
        for handle in handles:
            handle.remove()
