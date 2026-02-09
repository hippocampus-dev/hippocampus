import argparse
import asyncio
import enum
import os

import httpx
import numpy
import openai

import cortex.llm.openai


class Distance(enum.StrEnum):
    Dot = "dot"
    Cosine = "cosine"
    Manhattan = "manhattan"
    Euclidean = "euclidean"


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("files", nargs=2)
    parser.add_argument(
        "--distance",
        choices=[str(e) for e in Distance],
        default=Distance.Cosine,
    )
    parser.add_argument(
        "--model",
        choices=[str(e) for e in cortex.llm.openai.model.EmbeddingModel],
        default=cortex.llm.openai.model.EmbeddingModel.ADA_V3_SMALL,
    )
    parser.add_argument(
        "--dimensions",
        type=int,
        default=cortex.llm.openai.model.OPENAI_VECTOR_SIZE,
    )
    args = parser.parse_args()

    a, b = args.files

    with open(a, "r") as f:
        text_a = f.read()
    with open(b, "r") as f:
        text_b = f.read()

    results = await asyncio.gather(*[
        cortex.llm.openai.AsyncOpenAI(
            http_client=httpx.AsyncClient(timeout=None, mounts={
                "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
                "https://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTPS_PROXY")),
            }, verify=os.getenv("SSL_CERT_FILE")),
        ).embeddings.create(
            input=text,
            model=args.model,
            dimensions=args.dimensions if args.model in (
                cortex.llm.openai.model.EmbeddingModel.ADA_V3_SMALL,
                cortex.llm.openai.model.EmbeddingModel.ADA_V3_LARGE,
            ) else openai._types.NOT_GIVEN,
        )
        for text in [text_a, text_b]
    ])
    vec_a, vec_b = [numpy.array(data.embedding) for result in results for data in result.data]

    match args.distance:
        case Distance.Dot:
            similarity = numpy.dot(vec_a, vec_b)
        case Distance.Cosine:
            similarity = numpy.dot(vec_a, vec_b) / (numpy.linalg.norm(vec_a) * numpy.linalg.norm(vec_b))
        case Distance.Manhattan:
            similarity = numpy.linalg.norm(vec_a - vec_b, ord=1)
        case Distance.Euclidean:
            similarity = numpy.linalg.norm(vec_a - vec_b)
        case _:
            raise NotImplementedError

    print(similarity)


if __name__ == "__main__":
    asyncio.run(main())
