# Brief: Add image gallery block

status: open
type: feature
id: example
date: 2026-08-08
author: Leo

## What

Add a new block type that allows content editors to display a grid of images.
The block should be definable by the developer (field definitions in PHP) and
editable by the admin (uploading/reordering images in the admin UI).

## Why

Several solicms clients have asked for a way to display photo galleries on their
pages. Currently they're working around this by embedding external services,
which is fragile and inconsistent. A native gallery block keeps everything in
one place and under the CMS's control.

## Out of scope

- Lightbox / fullscreen view (separate job if validated)
- Video support
- External image URLs — only uploaded images
- Image editing or cropping

## Notes

Keep the admin UI as simple as possible. Content editors should be able to
upload images and drag to reorder. That's it. No captions, no alt text fields
in this iteration (accessibility follow-up is a separate job).