import abc
import collections.abc
import dataclasses
import typing


class Moderator(abc.ABC):
    @abc.abstractmethod
    async def moderate(self, content: str) -> bool:
        raise NotImplementedError


@dataclasses.dataclass(frozen=True)
class GuardrailResult:
    tripwire: bool
    reason: str | None = None


class Guardrail(abc.ABC):
    @abc.abstractmethod
    async def check(
        self,
        messages: collections.abc.Sequence[collections.abc.Mapping[str, typing.Any]],
    ) -> GuardrailResult:
        raise NotImplementedError
