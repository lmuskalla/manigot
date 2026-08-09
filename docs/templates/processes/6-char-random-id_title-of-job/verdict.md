# Verdict: Add image gallery block

id: example
status: needs work
reviewer: Claude (@reviewer)
date: 2026-08-08

## Review

TASK-1: PASS
  GalleryBlock class follows existing block conventions. Field definitions clean.

TASK-2: PASS
  Migration is non-destructive. Rollback tested. No impact on existing data.

TASK-3: PASS
  Svelte component renders correctly. Responsive behaviour works as expected.
  Minor: component doesn't handle empty images array gracefully — renders
  empty grid container instead of nothing. Low priority but worth fixing.

TASK-4: PARTIAL
  Upload endpoint works. Tenant isolation is applied correctly on storage path.
  However, file type validation is missing — any file type is accepted.
  MIME type must be validated server-side before storage. Do not ship without this.

TASK-5: PASS
  Admin UI is appropriately simple. Drag-to-reorder works. Remove button works.
  Tested with 1, 5, and 20 images.

TASK-6: PASS
  Block registered correctly. Appears in block picker.

## Security

[HIGH] GalleryBlockController@upload accepts any file type without MIME validation.
An authenticated admin could upload a PHP file disguised as an image.
Even with tenant isolation on the path, this is a meaningful risk if the storage
disk is ever misconfigured or made public. Validate MIME type server-side against
an explicit allowlist (image/jpeg, image/png, image/webp, image/gif) before storing.

[LOW] No file size limit enforced server-side. Large uploads could be used to
exhaust storage. Add a max file size check in the controller.

## Overall

NEEDS WORK

Two issues must be fixed before this ships:
1. Server-side MIME type validation on image upload (HIGH — security)
2. Server-side file size limit on image upload (LOW — stability)

The empty state rendering in GalleryBlock.svelte (TASK-3) can be fixed in a
follow-up, it doesn't block shipping.