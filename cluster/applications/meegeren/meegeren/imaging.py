import numpy
import torch
import torch.nn.functional


def dilate(mask: numpy.ndarray, radius: int) -> numpy.ndarray:
    if radius <= 0:
        return mask
    dilated = numpy.zeros_like(mask)
    if not mask.any():
        return dilated
    left, top, right, bottom = bounding_box(mask)
    left, top = max(0, left - radius), max(0, top - radius)
    right, bottom = (
        min(mask.shape[1], right + radius),
        min(mask.shape[0], bottom + radius),
    )
    tensor = torch.from_numpy(mask[top:bottom, left:right])[None, None].float()
    # A square max filter is separable
    vertical = torch.nn.functional.max_pool2d(
        tensor, kernel_size=(2 * radius + 1, 1), stride=1, padding=(radius, 0)
    )
    horizontal = torch.nn.functional.max_pool2d(
        vertical, kernel_size=(1, 2 * radius + 1), stride=1, padding=(0, radius)
    )
    dilated[top:bottom, left:right] = horizontal[0, 0].numpy() > 0
    return dilated


def feather(mask: numpy.ndarray, radius: int) -> numpy.ndarray:
    if radius <= 0:
        return mask
    left, top, right, bottom = bounding_box(mask)
    half = min((radius + 1) // 2, (min(right - left, bottom - top) - 1) // 4)
    # Blurring mask shrunk by the blur radius keeps every weight within mask
    tensor = torch.from_numpy(~dilate(~mask, half))[None, None].float()
    # Pixels past the image are left out, so mask does not fade where it reaches the border
    vertical = torch.nn.functional.avg_pool2d(
        tensor,
        kernel_size=(2 * half + 1, 1),
        stride=1,
        padding=(half, 0),
        count_include_pad=False,
    )
    blurred = torch.nn.functional.avg_pool2d(
        vertical,
        kernel_size=(1, 2 * half + 1),
        stride=1,
        padding=(0, half),
        count_include_pad=False,
    )
    return blurred[0, 0].numpy()


def bounding_box(mask: numpy.ndarray) -> tuple[int, int, int, int]:
    rows = numpy.flatnonzero(mask.any(axis=1))
    columns = numpy.flatnonzero(mask.any(axis=0))
    return int(columns[0]), int(rows[0]), int(columns[-1]) + 1, int(rows[-1]) + 1


def rectangle(shape: tuple[int, int], box: tuple[int, int, int, int]) -> numpy.ndarray:
    left, top, right, bottom = box
    mask = numpy.zeros(shape, dtype=bool)
    mask[max(0, top) : max(0, bottom), max(0, left) : max(0, right)] = True
    return mask


def bottom(mask: numpy.ndarray) -> int:
    return int(numpy.flatnonzero(mask.any(axis=1))[-1])
