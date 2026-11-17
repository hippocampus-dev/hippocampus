import html
import json
import os

import numpy
import PIL.Image
import PIL.ImageOps

import lora.model
import lora.settings

s = lora.settings.Settings()

THUMBNAIL_SIZE = 512
COLORS = {
    lora.model.Region.WALL: (0, 120, 255),
    lora.model.Region.FLOOR: (0, 200, 0),
    lora.model.Region.EXCLUDED: (255, 0, 0),
}
SCRIPT = """
const rejected = new Set(REJECTED);
for (const figure of document.querySelectorAll("figure")) {
  figure.classList.toggle("rejected", rejected.has(figure.id));
  figure.addEventListener("click", (event) => {
    if (event.target.closest("a")) {
      return;
    }
    rejected.has(figure.id) ? rejected.delete(figure.id) : rejected.add(figure.id);
    figure.classList.toggle("rejected", rejected.has(figure.id));
  });
}
document.querySelector("button").addEventListener("click", () => {
  const anchor = document.createElement("a");
  anchor.href = URL.createObjectURL(new Blob([[...rejected].join("\\n") + "\\n"], {type: "text/plain"}));
  anchor.download = "rejected.txt";
  anchor.click();
});
"""


def main():
    directory = os.path.expanduser(s.dataset_directory)
    os.makedirs(os.path.join(directory, "review"), exist_ok=True)
    with open(os.path.join(directory, lora.model.MANIFEST)) as f:
        images = {
            image.sha256: image
            for image in (
                lora.model.ImageObject.model_validate_json(line) for line in f
            )
        }
    with open(os.path.join(directory, lora.model.REGIONS)) as f:
        regions = [lora.model.Regions.model_validate_json(line) for line in f]
    rejected = []
    if os.path.exists(os.path.join(directory, lora.model.REJECTED)):
        with open(os.path.join(directory, lora.model.REJECTED)) as f:
            rejected = [line.strip() for line in f if line.strip() != ""]

    figures = []
    for region in sorted(regions, key=lambda r: r.wall + r.floor):
        thumbnail = os.path.join(directory, "review", f"{region.sha256}.jpg")
        mask_path = os.path.join(directory, "masks", f"{region.sha256}.png")
        if not os.path.exists(thumbnail) or os.path.getmtime(
            thumbnail
        ) < os.path.getmtime(mask_path):
            photo = PIL.ImageOps.exif_transpose(
                PIL.Image.open(
                    os.path.join(directory, "images", f"{region.sha256}.jpg")
                )
            ).convert("RGB")
            photo.thumbnail((THUMBNAIL_SIZE, THUMBNAIL_SIZE))
            mask = numpy.asarray(
                PIL.Image.open(mask_path).resize(
                    photo.size, PIL.Image.Resampling.NEAREST
                )
            )
            overlay = numpy.asarray(photo).astype(numpy.float32)
            for value, color in COLORS.items():
                overlay[mask == value] = (
                    overlay[mask == value] + numpy.array(color, numpy.float32)
                ) / 2
            side_by_side = PIL.Image.new("RGB", (photo.width * 2, photo.height))
            side_by_side.paste(photo, (0, 0))
            side_by_side.paste(
                PIL.Image.fromarray(overlay.astype(numpy.uint8)), (photo.width, 0)
            )
            side_by_side.save(thumbnail)
        image = images[region.sha256]
        figures.append(
            f'<figure id="{region.sha256}"><img src="review/{region.sha256}.jpg" loading="lazy">'
            f"<figcaption>{image.width}x{image.height} wall {region.wall:.0%} floor {region.floor:.0%} excluded {region.excluded:.0%} "
            f'<a href="{html.escape(image.url)}" target="_blank" rel="noopener">source</a></figcaption></figure>'
        )

    with open(os.path.join(directory, "review.html"), "w") as f:
        f.write(
            "<!doctype html>\n"
            '<html lang="en"><head><meta charset="utf-8"><title>LoRA Dataset Review</title>'
            "<style>body{font-family:sans-serif}figure{display:inline-block;margin:4px;cursor:pointer}"
            "figure.rejected{opacity:.25}img{display:block;max-width:100%}"
            "button{position:sticky;top:0}</style></head><body>"
            f"<button>Download {lora.model.REJECTED}</button>"
            + "".join(figures)
            + f"<script>const REJECTED = {json.dumps(rejected)};{SCRIPT}</script>"
            "</body></html>\n"
        )


if __name__ == "__main__":
    main()
