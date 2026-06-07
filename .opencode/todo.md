# Mission: Fix all code review issues ✅

## M1: Fix Critical Issues ✅
### F1.1: Fix expandTilde bug in service.go (C3) | status: completed ✅
- [x] S1.1.1: Fix expandTilde to use path[2:] instead of path[1:] ✅

### F1.2: Export ParseSortKey from episode package (M2) | status: completed ✅
- [x] S1.2.1: Export ParseSortKey ✅

### F1.3: Add ExpandTilde to paths package (M1) | status: completed ✅
- [x] S1.3.1: Add ExpandTilde to paths package ✅
- [x] S1.3.2: Add tests for ExpandTilde ✅

### F1.4: Strip Unicode control chars from TitleToDir (I5) | status: completed ✅
- [x] S1.4.1: Add unicode.IsControl stripping to TitleToDir ✅

### F1.5: ffmpeg Execute uses io.Discard (M7) | status: completed ✅
- [x] S1.5.1: Replace os.Stdout/os.Stderr with io.Discard ✅

## M2: Verify All Fixes ✅
### F2.1: Run all tests with race detector ✅
- [x] S2.1.1: go vet ./... - PASS ✅
- [x] S2.1.2: go test -race ./... - all 14 suites PASS ✅