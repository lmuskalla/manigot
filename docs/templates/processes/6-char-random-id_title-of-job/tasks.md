# Tasks: Add image gallery block

id: example
status: in progress
analyst: Claude (@analyst)
date: 2026-08-08

## Task breakdown

TASK-1: Create GalleryBlock PHP class
files: app/Blocks/GalleryBlock.php
depends: none
risk: low — straightforward block definition following existing pattern

TASK-2: Create gallery block migration
files: database/migrations/xxxx_add_gallery_block_fields.php
depends: TASK-1
risk: medium — migration must not affect existing block data

TASK-3: Create Svelte GalleryBlock component
files: resources/js/blocks/GalleryBlock.svelte
depends: TASK-1
risk: low — frontend component, no backend interaction

TASK-4: Add image upload endpoint for gallery block
files: app/Http/Controllers/Admin/GalleryBlockController.php, routes/api.php
depends: TASK-1
risk: medium — file upload handling, ensure tenant isolation on stored files

TASK-5: Build admin UI for gallery block editing
files: resources/js/admin/blocks/GalleryBlockEditor.svelte
depends: TASK-3, TASK-4
risk: medium — drag-to-reorder needs careful UX, keep it simple

TASK-6: Register block in service provider
files: app/Providers/BlockServiceProvider.php
depends: TASK-1, TASK-2
risk: low — follows existing registration pattern