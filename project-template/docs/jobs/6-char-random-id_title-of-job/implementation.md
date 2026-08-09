# Implementation: Add image gallery block

id: example
status: in progress
developer: Claude (@developer)
date: 2026-08-08

## Summary

TASK-1 through TASK-6 implemented. Gallery block is functional end-to-end.
Images upload, persist per tenant, and render in the Svelte frontend.
Drag-to-reorder works via HTML5 drag events with order persisted on drop.

## Changes

app/Blocks/GalleryBlock.php
  — New block class defining fields: images (array of media IDs), columns (int, default 3)

database/migrations/2026_08_08_000001_add_gallery_block_fields.php
  — Adds gallery_block_images and gallery_block_columns to block_content table
  — Non-destructive, nullable columns with defaults

app/Http/Controllers/Admin/GalleryBlockController.php
  — POST /api/admin/blocks/gallery/upload — handles image upload, stores under tenant path
  — PUT /api/admin/blocks/gallery/{block}/order — persists reordered image array

routes/api.php
  — Added two gallery block routes under auth:sanctum + tenant middleware

app/Providers/BlockServiceProvider.php
  — Registered GalleryBlock in block registry

resources/js/blocks/GalleryBlock.svelte
  — Renders image grid, columns configurable via block field
  — Responsive: collapses to 2-col on mobile, 1-col on small mobile

resources/js/admin/blocks/GalleryBlockEditor.svelte
  — Upload button (multiple file select)
  — Drag-to-reorder with visual drag handle
  — Remove button per image
  — No captions per brief scope

## Known issues / follow-ups

- Alt text is missing on all gallery images — accessibility job needed
- Column count is editable by developer only (block definition), not by admin —
  worth a follow-up to expose as an admin-configurable field
- Large image uploads have no client-side size validation yet