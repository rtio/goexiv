package goexiv

// #cgo pkg-config: exiv2
// #include "helper.h"
// #include <stdlib.h>
import "C"

import (
	"runtime"
	"unsafe"
)

// XmpData contains all Xmp Data of an image.
type XmpData struct {
	img  *Image // We point to img to keep it alive
	data *C.Exiv2XmpData
}

// XmpDatum stores the info of one xmp datum.
type XmpDatum struct {
	data  *XmpData
	datum *C.Exiv2XmpDatum
}

// XmpDatumIterator wraps the respective C++ structure.
type XmpDatumIterator struct {
	data *XmpData
	iter *C.Exiv2XmpDatumIterator
}

func makeXmpData(img *Image, cdata *C.Exiv2XmpData) *XmpData {
	data := &XmpData{
		img,
		cdata,
	}

	runtime.SetFinalizer(data, func(x *XmpData) {
		C.exiv2_xmp_data_free(x.data)
	})

	return data
}

func makeXmpDatum(data *XmpData, cdatum *C.Exiv2XmpDatum) *XmpDatum {
	if cdatum == nil {
		return nil
	}

	datum := &XmpDatum{
		data,
		cdatum,
	}

	runtime.SetFinalizer(datum, func(x *XmpDatum) {
		C.exiv2_xmp_datum_free(x.datum)
	})

	return datum
}

// GetXmpData returns the XmpData of an Image.
func (i *Image) GetXmpData() *XmpData {
	return makeXmpData(i, C.exiv2_image_get_xmp_data(i.img))
}

// FindKey tries to find the specified key and returns its data.
// It returns an error if the key is invalid. If the key is not found, a
// nil pointer will be returned
func (d *XmpData) FindKey(key string) (*XmpDatum, error) {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	var cerr *C.Exiv2Error

	cdatum := C.exiv2_xmp_data_find_key(d.data, ckey, &cerr)

	if cerr != nil {
		err := makeError(cerr)
		C.exiv2_error_free(cerr)
		return nil, err
	}

	runtime.KeepAlive(d)
	return makeXmpDatum(d, cdatum), nil
}

func (d *XmpDatum) String() string {
	cstr := C.exiv2_xmp_datum_to_string(d.datum)
	defer C.free(unsafe.Pointer(cstr))

	return C.GoString(cstr)
}

func (i *Image) XmpStripKey(key string) error {
	return i.StripKey(XMP, key)
}

func (i *Image) SetXmpString(key, value string) error {
	return i.SetMetadataString(XMP, key, value)
}

func (d *XmpData) GetString(key string) (string, error) {
	datum, err := d.FindKey(key)
	if err != nil {
		return "", err
	}

	if datum == nil {
		return "", ErrMetadataKeyNotFound
	}

	return datum.String(), nil
}

func (i *Image) XmpStripMetadata(unless []string) error {
	xmpData := i.GetXmpData()
	for iter := xmpData.Iterator(); iter.HasNext(); {
		key := iter.Next().Key()
		// Skip unless
		if contains(key, unless) {
			continue
		}
		err := i.StripKey(XMP, key)
		if err != nil {
			return err
		}
	}
	return nil
}

// Iterator returns a new XmpDatumIterator to iterate over all IPTC data.
func (d *XmpData) Iterator() *XmpDatumIterator {
	return makeXmpDatumIterator(d, C.exiv2_xmp_data_iterator(d.data))
}

// HasNext returns true as long as the iterator has another datum to deliver.
func (i *XmpDatumIterator) HasNext() bool {
	return C.exiv2_xmp_data_iterator_has_next(i.iter) != 0
}

// Next returns the next XmpDatum of the iterator or nil if iterator has reached the end.
func (i *XmpDatumIterator) Next() *XmpDatum {
	return makeXmpDatum(i.data, C.exiv2_xmp_datum_iterator_next(i.iter))
}

func makeXmpDatumIterator(data *XmpData, cIter *C.Exiv2XmpDatumIterator) *XmpDatumIterator {
	datum := &XmpDatumIterator{data, cIter}

	runtime.SetFinalizer(datum, func(i *XmpDatumIterator) {
		C.exiv2_xmp_datum_iterator_free(i.iter)
	})

	return datum
}

// Key returns the XMP key of the datum.
func (d *XmpDatum) Key() string {
	return C.GoString(C.exiv2_xmp_datum_key(d.datum))
}
