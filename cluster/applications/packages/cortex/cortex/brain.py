import abc
import asyncio
import typing

import botocore.exceptions
import opentelemetry.context

import cortex.exceptions


class Brain(abc.ABC):
    @abc.abstractmethod
    async def save(self, key: str, body: bytes):
        raise NotImplementedError

    @abc.abstractmethod
    async def restore(self, key: str) -> bytes | None:
        raise NotImplementedError


class S3Brain(Brain):
    s3_client: typing.Any
    bucket: str

    def __init__(self, s3_client: typing.Any, bucket: str):
        self.s3_client = s3_client
        self.bucket = bucket

    async def save(self, key: str, body: bytes):
        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            return self.s3_client.put_object(
                Bucket=self.bucket,
                Key=key,
                Body=body,
                ContentType="application/json",
            )

        try:
            await asyncio.get_running_loop().run_in_executor(None, run_in_context, opentelemetry.context.get_current())
        except botocore.exceptions.ClientError as e:
            match e.response["ResponseMetadata"]["HTTPStatusCode"]:
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e
        except (
            botocore.exceptions.EndpointConnectionError,  # ECONNREFUSED
            botocore.exceptions.ConnectTimeoutError,
            botocore.exceptions.ReadTimeoutError,
            botocore.exceptions.ConnectionClosedError,  # ECONNRESET
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

    async def restore(self, key: str) -> bytes | None:
        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            return self.s3_client.get_object(
                Bucket=self.bucket,
                Key=key,
            )

        try:
            response = await asyncio.get_running_loop().run_in_executor(
                None,
                run_in_context,
                opentelemetry.context.get_current(),
            )
        except self.s3_client.exceptions.NoSuchKey:
            return None
        except botocore.exceptions.ClientError as e:
            match e.response["ResponseMetadata"]["HTTPStatusCode"]:
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e
        except (
            botocore.exceptions.EndpointConnectionError,  # ECONNREFUSED
            botocore.exceptions.ConnectTimeoutError,
            botocore.exceptions.ReadTimeoutError,
            botocore.exceptions.ConnectionClosedError,  # ECONNRESET
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e

        return response["Body"].read()
