// MIT License
//
// Copyright (c) 2019 kpango (Yusuke Kato)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package ngt provides implementation of Go API for https://github.com/yahoojapan/NGT
package ngt

/*
#cgo LDFLAGS: -lngt
#include <NGT/Capi.h>
#include <stdlib.h>
*/
import "C"

import (
	"os"
	"reflect"
	"sync"
	"unsafe"

	"github.com/vdaas/vald/internal/errors"
)

type (

	// NGT is core interface
	NGT interface {
		// Search returns search result as []SearchResult
		Search(vec []float64, size int, epsilon, radius float32) ([]SearchResult, error)

		// Insert returns NGT object id.
		// This only stores not indexing, you must call CreateIndex and SaveIndex.
		Insert(vec []float64) (uint, error)

		// InsertCommit returns NGT object id.
		// This stores and indexes at the same time.
		InsertCommit(vec []float64, poolSize uint32) (uint, error)

		// BulkInsert returns NGT object ids.
		// This only stores not indexing, you must call CreateIndex and SaveIndex.
		BulkInsert(vecs [][]float64) ([]uint, []error)

		// BulkInsertCommit returns NGT object ids.
		// This stores and indexes at the same time.
		BulkInsertCommit(vecs [][]float64, poolSize uint32) ([]uint, []error)

		// CreateAndSaveIndex call  CreateIndex and SaveIndex in a row.
		CreateAndSaveIndex(poolSize uint32) error

		// CreateIndex creates NGT index.
		CreateIndex(poolSize uint32) error

		// SaveIndex stores NGT index to storage.
		SaveIndex() error

		// Remove removes from NGT index.
		Remove(id uint) error

		// GetVector returns vector stored in NGT index.
		GetVector(id uint) ([]float64, error)

		// Close NGT index.
		Close()
	}

	ngt struct {
		idxPath             string
		bulkInsertChunkSize int
		dimension           C.int32_t
		objectType          objectType
		prop                C.NGTProperty
		ebuf                C.NGTError
		index               C.NGTIndex
		ospace              C.NGTObjectSpace
		mu                  *sync.RWMutex
	}
)

// ObjectType is alias of object type in NGT
type objectType int

// DistanceType is alias of distance type in NGT
type distanceType int

const (
	// ObjectNone is unknown object type
	ObjectNone objectType = iota
	// Uint8 is 8bit unsigned integer
	Uint8
	// Float is 32bit floating point number
	Float

	// DistanceNone is unknown distance type
	DistanceNone distanceType = iota - 1
	// L1 is l1 norm
	L1
	// L2 is l2 norm
	L2
	// Angle is angle distance
	Angle
	// Hamming is hamming distance
	Hamming
	// Cosine is cosine distance
	Cosine
	// NormalizedAngle is angle distance with normalization
	NormalizedAngle
	// NormalizedCosine is cosine distance with normalization
	NormalizedCosine
)

// New returns NGT instance with recreating empty index file
func New(opts ...Option) (NGT, error) {
	return gen(false, opts...)
}

// Load returns NGT instance from existing index file
func Load(opts ...Option) (NGT, error) {
	return gen(true, opts...)
}

func gen(isLoad bool, opts ...Option) (NGT, error) {
	var (
		n   = new(ngt)
		err error
	)
	n.mu = new(sync.RWMutex)

	defer func() {
		if err != nil {
			n.Close()
		}
	}()

	err = n.setup()
	if err != nil {
		return nil, err
	}

	defer C.ngt_destroy_property(n.prop)

	err = n.loadOptions(opts...)
	if err != nil {
		return nil, err
	}

	if isLoad {
		err = n.open()
	} else {
		err = n.create()
	}

	if err != nil {
		return nil, err
	}

	err = n.loadObjectSpace()
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (n *ngt) setup() error {
	n.ebuf = C.ngt_create_error_object()

	n.prop = C.ngt_create_property(n.ebuf)
	if n.prop == nil {
		return errors.ErrCreateProperty(newGoError(n.ebuf))
	}
	return nil
}

func (n *ngt) loadOptions(opts ...Option) (err error) {
	for _, opt := range append(defaultOpts, opts...) {
		err = opt(n)
		if err != nil {
			err = errors.ErrOptionFailed(err, reflect.ValueOf(opt))
			return err
		}
	}
	return nil
}

func (n *ngt) create() (err error) {
	if fileExists(n.idxPath) {
		if err = os.Remove(n.idxPath); err != nil {
			return err
		}
	}

	n.index = C.ngt_create_graph_and_tree(C.CString(n.idxPath), n.prop, n.ebuf)
	if n.index == nil {
		return newGoError(n.ebuf)
	}
	if C.ngt_save_index(n.index, C.CString(n.idxPath), n.ebuf) == ErrorCode {
		return newGoError(n.ebuf)
	}
	return nil
}

func (n *ngt) open() error {
	if !fileExists(n.idxPath) {
		return errors.ErrIndexNotFound
	}

	n.index = C.ngt_open_index(C.CString(n.idxPath), n.ebuf)
	if n.index == nil {
		return newGoError(n.ebuf)
	}

	if C.ngt_get_property(n.index, n.prop, n.ebuf) == ErrorCode {
		return newGoError(n.ebuf)
	}

	n.dimension = C.ngt_get_property_dimension(n.prop, n.ebuf)
	if int(n.dimension) == -1 {
		return newGoError(n.ebuf)
	}
	return nil
}

func (n *ngt) loadObjectSpace() error {
	n.ospace = C.ngt_get_object_space(n.index, n.ebuf)
	if n.ospace == nil {
		return newGoError(n.ebuf)
	}
	return nil
}

// Search returns search result as []SearchResult
func (n *ngt) Search(vec []float64, size int, epsilon, radius float32) ([]SearchResult, error) {

	results := C.ngt_create_empty_results(n.ebuf)
	defer C.ngt_destroy_results(results)
	if results == nil {
		return nil, newGoError(n.ebuf)
	}

	n.mu.RLock()

	ret := C.ngt_search_index(n.index,
		(*C.double)(&vec[0]),
		n.dimension,
		// C.size_t(size),
		*(*C.size_t)(unsafe.Pointer(&size)),
		// C.float(epsilon),
		*(*C.float)(unsafe.Pointer(&epsilon)),
		*(*C.float)(unsafe.Pointer(&radius)),
		// C.float(radius),
		results,
		n.ebuf)

	n.mu.RUnlock()

	if ret == ErrorCode {
		return nil, newGoError(n.ebuf)
	}

	rsize := int(C.ngt_get_size(results, n.ebuf))
	if rsize == -1 {
		return nil, newGoError(n.ebuf)
	}

	result := make([]SearchResult, rsize)
	for i := 0; i < rsize; i++ {
		d := C.ngt_get_result(results, C.uint32_t(i), n.ebuf)
		if d.id == 0 && d.distance == 0 {
			result[i] = SearchResult{0, 0, newGoError(n.ebuf)}
		} else {
			result[i] = SearchResult{uint32(d.id), float32(d.distance), nil}
		}
	}

	return result, nil
}

// Insert returns NGT object id.
// This only stores not indexing, you must call CreateIndex and SaveIndex.
func (n *ngt) Insert(vec []float64) (uint, error) {
	n.mu.Lock()
	id := C.ngt_insert_index(n.index, (*C.double)(&vec[0]), C.uint32_t(n.dimension), n.ebuf)
	n.mu.Unlock()
	if id == 0 {
		return 0, newGoError(n.ebuf)
	}

	return uint(id), nil
}

// InsertCommit returns NGT object id.
// This stores and indexes at the same time.
func (n *ngt) InsertCommit(vec []float64, poolSize uint32) (uint, error) {
	id, err := n.Insert(vec)
	if err != nil {
		return id, err
	}

	err = n.CreateIndex(poolSize)
	if err != nil {
		return id, err
	}

	err = n.SaveIndex()
	if err != nil {
		return id, err
	}

	return id, nil
}

// BulkInsert returns NGT object ids.
// This only stores not indexing, you must call CreateIndex and SaveIndex.
func (n *ngt) BulkInsert(vecs [][]float64) ([]uint, []error) {
	ids := make([]uint, 0, len(vecs))
	errs := make([]error, 0, len(vecs))

	var id uint

	n.mu.Lock()
	for _, vec := range vecs {
		// n.mu.Lock()
		id = uint(C.ngt_insert_index(n.index, (*C.double)(&vec[0]), C.uint32_t(n.dimension), n.ebuf))
		// n.mu.Unlock()
		if id == 0 {
			errs = append(errs, newGoError(n.ebuf))
		} else {
			ids = append(ids, uint(id))
		}
	}
	n.mu.Unlock()

	return ids, errs
}

// BulkInsertCommit returns NGT object ids.
// This stores and indexes at the same time.
func (n *ngt) BulkInsertCommit(vecs [][]float64, poolSize uint32) ([]uint, []error) {
	ids := make([]uint, 0, len(vecs))
	errs := make([]error, 0, len(vecs))

	idx := 0
	var id uint
	var err error

	for _, vec := range vecs {
		if id, err = n.Insert(vec); err == nil {
			ids = append(ids, id)
			idx++
			if idx >= n.bulkInsertChunkSize {
				err = n.CreateAndSaveIndex(poolSize)
				if err != nil {
					errs = append(errs, err)
				}
				idx = 0
			}
		} else {
			errs = append(errs, err)
		}
	}

	if idx > 0 {
		err = n.CreateAndSaveIndex(poolSize)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return ids, errs
}

// CreateAndSaveIndex call  CreateIndex and SaveIndex in a row.
func (n *ngt) CreateAndSaveIndex(poolSize uint32) error {
	err := n.CreateIndex(poolSize)
	if err != nil {
		return err
	}
	return n.SaveIndex()
}

// CreateIndex creates NGT index.
func (n *ngt) CreateIndex(poolSize uint32) error {
	n.mu.Lock()
	ret := C.ngt_create_index(n.index, C.uint32_t(poolSize), n.ebuf)
	n.mu.Unlock()
	if ret == ErrorCode {
		return newGoError(n.ebuf)
	}

	return nil
}

// SaveIndex stores NGT index to storage.
func (n *ngt) SaveIndex() error {
	n.mu.RLock()
	ret := C.ngt_save_index(n.index, C.CString(n.idxPath), n.ebuf)
	n.mu.RUnlock()

	if ret == ErrorCode {
		return newGoError(n.ebuf)
	}

	return nil
}

// Remove removes from NGT index.
func (n *ngt) Remove(id uint) error {
	n.mu.Lock()
	ret := C.ngt_remove_index(n.index, C.ObjectID(id), n.ebuf)
	n.mu.Unlock()
	if ret == ErrorCode {
		return newGoError(n.ebuf)
	}

	return nil
}

// GetVector returns vector stored in NGT index.
func (n *ngt) GetVector(id uint) ([]float64, error) {
	dimension := int(n.dimension)
	ret := make([]float64, 0, dimension)
	switch n.objectType {
	case Float:
		n.mu.RLock()
		results := C.ngt_get_object_as_float(n.ospace, C.ObjectID(id), n.ebuf)
		n.mu.RUnlock()
		if results == nil {
			return nil, newGoError(n.ebuf)
		}
		slice := (*[1 << 30]C.float)(unsafe.Pointer(results))[:dimension:dimension]
		for i := 0; i < dimension; i++ {
			ret[i] = float64(slice[i])
		}
	case Uint8:
		n.mu.RLock()
		results := C.ngt_get_object_as_integer(n.ospace, C.ObjectID(id), n.ebuf)
		n.mu.RUnlock()
		if results == nil {
			return nil, newGoError(n.ebuf)
		}
		slice := (*[1 << 30]C.uchar)(unsafe.Pointer(results))[:dimension:dimension]
		for i := 0; i < dimension; i++ {
			ret[i] = float64(slice[i])
		}
	default:
		return nil, errors.ErrUnsupportedObjectType
	}
	return ret, nil
}

func (n *ngt) refreshEbufIfError(err error) {
	if err != nil {
		C.ngt_destroy_error_object(n.ebuf)
		n.ebuf = C.ngt_create_error_object()
	}
}

// Close NGT index.
func (n *ngt) Close() {
	if n.index != nil {
		C.ngt_close_index(n.index)
		C.ngt_destroy_error_object(n.ebuf)
		n.index = nil

		// dimension           C.int32_t
		// objectType          objectType
		// prop                C.NGTProperty
		// ebuf                C.NGTError
		// index               C.NGTIndex
		// ospace              C.NGTObjectSpace
		C.free(unsafe.Pointer(&n.dimension))
		C.free(unsafe.Pointer(&n.prop))
		C.free(unsafe.Pointer(&n.ebuf))
		C.free(unsafe.Pointer(&n.ospace))
	}
}
