import abc


class Moderator(abc.ABC):
    @abc.abstractmethod
    async def moderate(self, content: str) -> bool:
        raise NotImplementedError
