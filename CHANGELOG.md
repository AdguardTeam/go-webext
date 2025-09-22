# go-webext changelog

## 0.2.1 (2025-09-22)

### Changed
- Timeout for Chrome Web Store API requests for `publish` command increased
  to 120 seconds to handle skip review check.

## 0.2.0 (2025-02-07)

### Added
- Support for Microsoft Edge Store API v1.1
- Chrome Web Store publish options: gradual rollout, expedited review, and trusted testers target

### Changed
- Marked Microsoft Edge Store API v1.0 as deprecated (will be removed by Microsoft soon)
- Updated documentation for Edge Store API credentials

## 0.1.3 (2024-02-28)

### Changed
- Download already signed files instead of uploading again.

## 0.1.2 (2023-10-19)

### Changed
- Fix downloading signed xpi file.

## 0.1.1 (2023-10-10)

### Changed
- Added output field to specify where to save signed xpi file.

## 0.1.0 (2023-07-04)

### Changed
- We've migrated from using the v4 API to the v5 API of AMO.
