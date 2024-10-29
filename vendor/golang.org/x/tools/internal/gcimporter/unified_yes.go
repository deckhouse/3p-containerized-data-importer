// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

<<<<<<< HEAD
//go:build goexperiment.unified
// +build goexperiment.unified
=======
//go:build go1.18 && goexperiment.unified
// +build go1.18,goexperiment.unified
>>>>>>> b3ea800a0 (feat: add image exporter (#1))

package gcimporter

const unifiedIR = true
