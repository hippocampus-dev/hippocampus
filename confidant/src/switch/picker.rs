use ab_glyph::{Font, ScaleFont};

// Font::pt_to_px_scale assumes this resolution.
const FONT_DPI: f32 = 96.0;

struct Icon<'a> {
    width: usize,
    height: usize,
    pixels: &'a [u32],
}

struct Row<'a> {
    title: &'a str,
    icon: Option<Icon<'a>>,
}

pub struct Picker<'a> {
    font: ab_glyph::FontVec,
    scale: ab_glyph::PxScale,
    rows: Vec<Row<'a>>,
    icon_size: usize,
    row_height: usize,
    pub width: u16,
    pub height: u16,
}

fn blend(pixels: &mut [u8], pixel: usize, color: u32, alpha: f32) {
    for channel in 0..3 {
        let background = pixels[pixel + channel] as f32;
        let foreground = (color >> (channel * 8) & 0xff) as f32;
        pixels[pixel + channel] = (background + (foreground - background) * alpha) as u8;
    }
}

impl<'a> Picker<'a> {
    /// Takes the title of each row and its `_NET_WM_ICON`, the width and height of each size followed by its ARGB pixels.
    pub fn new(
        dots_per_inch: f32,
        windows: Vec<(&'a str, &'a [u32])>,
        maximum_width: i32,
    ) -> Result<Self, error::Error> {
        let output = std::process::Command::new("fc-match")
            .args(["--format=%{index}|%{size}|%{file}", "sans:bold"])
            .output()?;
        if !output.status.success() {
            error::bail!("fc-match failed: {}", output.status);
        }
        let output = String::from_utf8(output.stdout)?;
        let [index, points, file] = output.splitn(3, '|').collect::<Vec<_>>()[..] else {
            error::bail!("fc-match printed {output}");
        };
        let font = ab_glyph::FontVec::try_from_vec_and_index(std::fs::read(file)?, index.parse()?)?;
        let Some(scale) = font.pt_to_px_scale(points.parse::<f32>()? * dots_per_inch / FONT_DPI)
        else {
            error::bail!("{file} has no units per em");
        };

        let icon_size = (super::ICON_SIZE * dots_per_inch / FONT_DPI) as usize;
        let rows = windows
            .into_iter()
            .map(|(title, mut data)| {
                let mut icon: Option<Icon> = None;
                while let [width, height, rest @ ..] = data {
                    let (width, height) = (*width as usize, *height as usize);
                    if rest.len() < width * height {
                        break;
                    }
                    let (pixels, next) = rest.split_at(width * height);
                    data = next;
                    // The smallest size at least as large as the icon, or else the largest.
                    let better = icon.as_ref().is_none_or(|icon| {
                        if icon.width >= icon_size {
                            width >= icon_size && width < icon.width
                        } else {
                            width > icon.width
                        }
                    });
                    if better {
                        icon = Some(Icon {
                            width,
                            height,
                            pixels,
                        });
                    }
                }
                Row { title, icon }
            })
            .collect::<Vec<_>>();

        let scaled = font.as_scaled(scale);
        let widest = rows
            .iter()
            .map(|row| {
                row.title
                    .chars()
                    .map(|character| scaled.h_advance(scaled.glyph_id(character)))
                    .sum::<f32>()
            })
            .fold(0.0, f32::max);
        let hilite_size = icon_size + super::HILITE_OFFSET * 2;
        let row_height =
            (scaled.height().ceil() as usize + super::TEXT_BORDER * 2).max(hilite_size);
        let width = super::OUTSIDE_BORDER * 3
            + hilite_size
            + super::TEXT_BORDER * 2
            + widest.ceil() as usize;
        let height = super::OUTSIDE_BORDER * 2 + row_height * rows.len();
        Ok(Self {
            font,
            scale,
            rows,
            icon_size,
            row_height,
            width: width.min(maximum_width as usize) as u16,
            height: height as u16,
        })
    }

    /// Returns the rows of little-endian 32-bit pixels showing `selected` highlighted.
    pub fn render(&self, selected: usize) -> Vec<u8> {
        let width = self.width as usize;
        let height = self.height as usize;
        let scaled = self.font.as_scaled(self.scale);
        let hilite_size = self.icon_size + super::HILITE_OFFSET * 2;
        let mut pixels = vec![0; width * height * 4];
        for pixel in (0..pixels.len()).step_by(4) {
            blend(&mut pixels, pixel, super::BACKGROUND, 1.0);
        }
        for (index, row) in self.rows.iter().enumerate() {
            let top = super::OUTSIDE_BORDER + self.row_height * index;
            let hilite_top = top + (self.row_height - hilite_size) / 2;
            if index == selected {
                for y in 0..hilite_size {
                    for x in 0..hilite_size {
                        let ring = x < super::HILITE_WIDTH
                            || y < super::HILITE_WIDTH
                            || x >= hilite_size - super::HILITE_WIDTH
                            || y >= hilite_size - super::HILITE_WIDTH;
                        let alpha = if ring {
                            super::HILITE_RING_ALPHA
                        } else {
                            super::HILITE_FILL_ALPHA
                        };
                        let pixel = ((hilite_top + y) * width + super::OUTSIDE_BORDER + x) * 4;
                        blend(&mut pixels, pixel, super::ACTIVE_TEXT, alpha as f32 / 255.0);
                    }
                }
            }
            if let Some(icon) = &row.icon {
                for y in 0..self.icon_size {
                    for x in 0..self.icon_size {
                        let source = icon.pixels[y * icon.height / self.icon_size * icon.width
                            + x * icon.width / self.icon_size];
                        let pixel = ((hilite_top + super::HILITE_OFFSET + y) * width
                            + super::OUTSIDE_BORDER
                            + super::HILITE_OFFSET
                            + x)
                            * 4;
                        blend(&mut pixels, pixel, source, (source >> 24) as f32 / 255.0);
                    }
                }
            }

            let color = if index == selected {
                super::ACTIVE_TEXT
            } else {
                super::INACTIVE_TEXT
            };
            let baseline =
                top as f32 + (self.row_height as f32 - scaled.height()) / 2.0 + scaled.ascent();
            let mut pen = (super::OUTSIDE_BORDER * 2 + hilite_size + super::TEXT_BORDER) as f32;
            for character in row.title.chars() {
                let id = scaled.glyph_id(character);
                let glyph = id.with_scale_and_position(self.scale, ab_glyph::point(pen, baseline));
                pen += scaled.h_advance(id);
                if pen > (width - super::OUTSIDE_BORDER - super::TEXT_BORDER) as f32 {
                    break;
                }
                let Some(outline) = self.font.outline_glyph(glyph) else {
                    continue;
                };
                let bounds = outline.px_bounds();
                outline.draw(|x, y, coverage| {
                    let x = bounds.min.x as usize + x as usize;
                    let y = bounds.min.y as usize + y as usize;
                    if x < width && y < height {
                        blend(&mut pixels, (y * width + x) * 4, color, coverage);
                    }
                });
            }
        }
        pixels
    }
}
