package goexiv_test

import (
	"github.com/rtio/goexiv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"runtime"
	"sync"
	"testing"
)

func TestOpenImage(t *testing.T) {
	// Open valid file
	img, err := goexiv.Open("testdata/pixel.jpg")

	if err != nil {
		t.Fatalf("Cannot open image: %s", err)
	}

	if img == nil {
		t.Fatalf("img is nil after successful open")
	}

	// Open non existing file

	img, err = goexiv.Open("thisimagedoesnotexist")

	if err == nil {
		t.Fatalf("No error set after opening a non existing image")
	}

	exivErr, ok := err.(*goexiv.Error)

	if !ok {
		t.Fatalf("Returned error is not of type Error")
	}

	if exivErr.Code() != 9 {
		t.Fatalf("Unexpected error code (expected 9, got %d)", exivErr.Code())
	}
}

func Test_OpenBytes(t *testing.T) {
	bytes, err := os.ReadFile("testdata/pixel.jpg")
	require.NoError(t, err)

	img, err := goexiv.OpenBytes(bytes)
	if assert.NoError(t, err) {
		assert.NotNil(t, img)
	}
}

func Test_OpenBytesFailures(t *testing.T) {
	tests := []struct {
		name        string
		bytes       []byte
		wantErr     string
		wantErrCode int
	}{
		{
			"no image",
			[]byte("no image"),
			"Failed to read input data",
			20,
		},
		{
			"empty byte slice",
			[]byte{},
			"input is empty",
			0,
		},
		{
			"nil byte slice",
			nil,
			"input is empty",
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := goexiv.OpenBytes(tt.bytes)
			if assert.EqualError(t, err, tt.wantErr) {
				exivErr, ok := err.(*goexiv.Error)
				if assert.True(t, ok, "occurred error is not of Type goexiv.Error") {
					assert.Equal(t, tt.wantErrCode, exivErr.Code(), "unexpected error code")
				}
			}
		})
	}
}

func TestMetadata(t *testing.T) {
	initializeImage("testdata/pixel.jpg", t)
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.ReadMetadata()

	if err != nil {
		t.Fatalf("Cannot read image metadata: %s", err)
	}

	width := img.PixelWidth()
	height := img.PixelHeight()
	if width != 1 || height != 1 {
		t.Errorf("Cannot read image size (expected 1x1, got %dx%d)", width, height)
	}

	data := img.GetExifData()

	// Invalid key
	datum, err := data.FindKey("NotARealKey")

	if err == nil {
		t.Fatalf("FindKey returns a nil error for an invalid key")
	}

	if datum != nil {
		t.Fatalf("FindKey does not return nil for an invalid key")
	}

	// Valid, existing key

	datum, err = data.FindKey("Exif.Image.Make")

	if err != nil {
		t.Fatalf("FindKey returns an error for a valid, existing key: %s", err)
	}

	if datum == nil {
		t.Fatalf("FindKey returns nil for a valid, existing key")
	}

	if datum.String() != "FakeMake" {
		t.Fatalf("Unexpected value for EXIF datum Exif.Image.Make (expected 'FakeMake', got '%s')", datum.String())
	}

	// Valid, non existing key

	datum, err = data.FindKey("Exif.Photo.Flash")

	if err != nil {
		t.Fatalf("FindKey returns an error for a valid, non existing key: %s", err)
	}

	if datum != nil {
		t.Fatalf("FindKey returns a non null datum for a valid, non existing key")
	}

	assert.Equal(t, map[string]string{
		"Exif.Image.Artist":                  "John Doe",
		"Exif.Image.Copyright":               "©2023 John Doe, all rights reserved",
		"Exif.Image.ExifTag":                 "202",
		"Exif.Image.Make":                    "FakeMake",
		"Exif.Image.Model":                   "FakeModel",
		"Exif.Image.ResolutionUnit":          "2",
		"Exif.Image.XResolution":             "72/1",
		"Exif.Image.YCbCrPositioning":        "1",
		"Exif.Image.YResolution":             "72/1",
		"Exif.Photo.ColorSpace":              "65535",
		"Exif.Photo.ComponentsConfiguration": "1 2 3 0",
		"Exif.Photo.DateTimeDigitized":       "2013:12:08 21:06:10",
		"Exif.Photo.ExifVersion":             "48 50 51 48",
		"Exif.Photo.FlashpixVersion":         "48 49 48 48",
	}, data.AllTags())

	//
	// IPTC
	//
	iptcData := img.GetIptcData()
	assert.Equal(t, map[string]string{
		"Iptc.Application2.Copyright":   "this is the copy, right?",
		"Iptc.Application2.CountryName": "Lancre",
		"Iptc.Application2.DateCreated": "2012-10-13",
		"Iptc.Application2.TimeCreated": "12:49:32+01:00",
	}, iptcData.AllTags())

	//
	// XMP
	//
	xmpData := img.GetXmpData()
	assert.Equal(t, map[string]string{
		"Xmp.iptc.CopyrightNotice": "this is the copy, right?",
		"Xmp.iptc.CreditLine":      "John Doe",
		"Xmp.iptc.JobId":           "12345",
	}, xmpData.AllTags())
}

func TestNoMetadata(t *testing.T) {
	img, err := goexiv.Open("testdata/stripped_pixel.jpg")
	require.NoError(t, err)
	err = img.ReadMetadata()
	require.NoError(t, err)
	assert.Nil(t, img.ICCProfile())
}

type MetadataTestCase struct {
	Format                 goexiv.MetadataFormat
	Key                    string
	Value                  string
	ImageFilename          string
	ExpectedErrorSubstring string
}

var metadataSetStringTestCases = []MetadataTestCase{
	// valid exif key, jpeg
	{
		Format:                 goexiv.EXIF,
		Key:                    "Exif.Photo.UserComment",
		Value:                  "Hello, world! Привет, мир!",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "", // no error
	},
	// valid exif key, webp
	{
		Format:                 goexiv.EXIF,
		Key:                    "Exif.Photo.UserComment",
		Value:                  "Hello, world! Привет, мир!",
		ImageFilename:          "testdata/pixel.webp",
		ExpectedErrorSubstring: "",
	},
	// valid iptc key, jpeg.
	// webp iptc is not supported (see libexiv2/src/webpimage.cpp WebPImage::setIptcData))
	{
		Format:                 goexiv.IPTC,
		Key:                    "Iptc.Application2.Caption",
		Value:                  "Hello, world! Привет, мир!",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "",
	},
	// invalid exif key, jpeg
	{
		Format:                 goexiv.IPTC,
		Key:                    "Exif.Invalid.Key",
		Value:                  "this value should not be written",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "Invalid key",
	},
	{
		Format:                 goexiv.XMP,
		Key:                    "Xmp.iptc.CreditLine",
		Value:                  "Hello, world!",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "",
	},
	// invalid exif key, webp
	{
		Format:                 goexiv.EXIF,
		Key:                    "Exif.Invalid.Key",
		Value:                  "this value should not be written",
		ImageFilename:          "testdata/pixel.webp",
		ExpectedErrorSubstring: "Invalid key",
	},
	// invalid iptc key, jpeg
	{
		Format:                 goexiv.IPTC,
		Key:                    "Iptc.Invalid.Key",
		Value:                  "this value should not be written",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "Invalid record name",
	},
	// invalid exif key, jpeg
	{
		Format:                 goexiv.XMP,
		Key:                    "Xmp.Invalid.Key",
		Value:                  "this value should not be written",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "No namespace info available for XMP prefix",
	},
}

func Test_SetMetadataStringFromFile(t *testing.T) {
	var data goexiv.MetadataProvider

	for i, testcase := range metadataSetStringTestCases {
		img, err := goexiv.Open(testcase.ImageFilename)
		require.NoErrorf(t, err, "case #%d Error while opening image file", i)

		err = img.SetMetadataString(testcase.Format, testcase.Key, testcase.Value)
		if testcase.ExpectedErrorSubstring != "" {
			require.Errorf(t, err, "case #%d Error was expected", i)
			require.Containsf(
				t,
				err.Error(),
				testcase.ExpectedErrorSubstring,
				"case #%d Error text must contain a given substring",
				i,
			)
			continue
		}

		require.NoErrorf(t, err, "case #%d Cannot write image metadata", i)

		err = img.ReadMetadata()
		require.NoErrorf(t, err, "case #%d Cannot read image metadata", i)

		switch testcase.Format {
		case goexiv.EXIF:
			data = img.GetExifData()
		case goexiv.IPTC:
			data = img.GetIptcData()
		case goexiv.XMP:
			data = img.GetXmpData()
		}

		receivedValue, err := data.GetString(testcase.Key)
		require.Equalf(
			t,
			testcase.Value,
			receivedValue,
			"case #%d Value written must be equal to the value read",
			i,
		)
	}
}

// TesSetMetadataString when metadata format is invalid
func Test_SetMetadataStringInvalidFormat(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.SetMetadataString(999, "Exif.Photo.UserComment", "Hello, world!")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid metadata type")
}

// TestSetMetadataShort when metadata format is invalid
func Test_SetMetadataShortInvalidFormat(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.SetMetadataShort(999, "Exif.Photo.ExposureProgram", "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid metadata type")
}

var metadataSetShortIntTestCases = []MetadataTestCase{
	// valid exif key, jpeg
	{
		Format:                 goexiv.EXIF,
		Key:                    "Exif.Photo.ExposureProgram",
		Value:                  "1",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "", // no error
	},
	// valid exif key, webp
	{
		Format:                 goexiv.EXIF,
		Key:                    "Exif.Photo.ExposureProgram",
		Value:                  "2",
		ImageFilename:          "testdata/pixel.webp",
		ExpectedErrorSubstring: "",
	},
	// valid iptc key, jpeg.
	// webp iptc is not supported (see libexiv2/src/webpimage.cpp WebPImage::setIptcData))
	{
		Format:                 goexiv.IPTC,
		Key:                    "Iptc.Envelope.ModelVersion",
		Value:                  "3",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "",
	},
	// invalid exif key, jpeg
	{
		Format:                 goexiv.EXIF,
		Key:                    "Exif.Invalid.Key",
		Value:                  "4",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "Invalid key",
	},
	// invalid exif key, webp
	{
		Format:                 goexiv.EXIF,
		Key:                    "Exif.Invalid.Key",
		Value:                  "5",
		ImageFilename:          "testdata/pixel.webp",
		ExpectedErrorSubstring: "Invalid key",
	},
	// invalid iptc key, jpeg
	{
		Format:                 goexiv.IPTC,
		Key:                    "Iptc.Invalid.Key",
		Value:                  "6",
		ImageFilename:          "testdata/pixel.jpg",
		ExpectedErrorSubstring: "Invalid record name",
	},
}

func Test_SetMetadataShortInt(t *testing.T) {
	var data goexiv.MetadataProvider

	for i, testcase := range metadataSetShortIntTestCases {
		img, err := goexiv.Open(testcase.ImageFilename)
		require.NoErrorf(t, err, "case #%d Error while opening image file", i)

		err = img.SetMetadataShort(testcase.Format, testcase.Key, testcase.Value)
		if testcase.ExpectedErrorSubstring != "" {
			require.Errorf(t, err, "case #%d Error was expected", i)
			require.Containsf(
				t,
				err.Error(),
				testcase.ExpectedErrorSubstring,
				"case #%d Error text must contain a given substring",
				i,
			)
			continue
		}

		require.NoErrorf(t, err, "case #%d Cannot write image metadata", i)

		err = img.ReadMetadata()
		require.NoErrorf(t, err, "case #%d Cannot read image metadata", i)

		if testcase.Format == goexiv.IPTC {
			data = img.GetIptcData()
		} else {
			data = img.GetExifData()
		}

		receivedValue, err := data.GetString(testcase.Key)
		require.Equalf(
			t,
			testcase.Value,
			receivedValue,
			"case #%d Value written must be equal to the value read",
			i,
		)
	}
}

// Test SetIptcShort
func Test_SetIptcShort(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.SetIptcShort("Iptc.Envelope.ModelVersion", "1")
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	data := img.GetIptcData()
	receivedValue, err := data.GetString("Iptc.Envelope.ModelVersion")
	require.Equal(t, "1", receivedValue)
}

func Test_GetBytes(t *testing.T) {
	bytes, err := os.ReadFile("testdata/stripped_pixel.jpg")
	require.NoError(t, err)

	img, err := goexiv.OpenBytes(bytes)
	require.NoError(t, err)

	require.Equal(
		t,
		len(bytes),
		len(img.GetBytes()),
		"Image size on disk and in memory must be equal",
	)

	bytesBeforeTag := img.GetBytes()
	assert.NoError(t, img.SetExifString("Exif.Photo.UserComment", "123"))
	bytesAfterTag := img.GetBytes()
	assert.True(t, len(bytesAfterTag) > len(bytesBeforeTag), "Image size must increase after adding an EXIF tag")
	assert.Equal(t, &bytesBeforeTag[0], &bytesAfterTag[0], "Every call to GetBytes must point to the same underlying array")

	assert.NoError(t, img.SetExifString("Exif.Photo.UserComment", "123"))
	bytesAfterTag2 := img.GetBytes()
	assert.Equal(
		t,
		len(bytesAfterTag),
		len(bytesAfterTag2),
		"Image size must not change after the same tag has been set",
	)
}

// Ensures image manipulation doesn't fail when running from multiple goroutines
func Test_GetBytes_Goroutine(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 0

	bytes, err := os.ReadFile("testdata/stripped_pixel.jpg")
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		iterations++
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			img, err := goexiv.OpenBytes(bytes)
			require.NoError(t, err)

			// trigger garbage collection to increase the chance that underlying img.img will be collected
			runtime.GC()

			bytesAfter := img.GetBytes()
			assert.NotEmpty(t, bytesAfter)

			// if this line is removed, then the test will likely fail
			// with segmentation violation.
			// so far we couldn't come up with a better solution.
			runtime.KeepAlive(img)
		}(i)
	}

	wg.Wait()
	runtime.GC()
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	t.Logf("Allocated bytes after test:  %+v\n", memStats.HeapAlloc)
}

// TestStripKey when metadata format is invalid
func TestStripKey_InvalidFormat(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.StripKey(999, "Exif.Photo.UserComment")
	require.Error(t, err)
}

func TestExifStripKey(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.SetExifString("Exif.Photo.UserComment", "123")
	require.NoError(t, err)

	err = img.ExifStripKey("Exif.Photo.UserComment")
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	data := img.GetExifData()

	_, err = data.GetString("Exif.Photo.UserComment")
	require.Error(t, err)
}

func TestIptcStripKey(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.SetIptcString("Iptc.Application2.Caption", "123")
	require.NoError(t, err)

	err = img.IptcStripKey("Iptc.Application2.Caption")
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	data := img.GetIptcData()

	_, err = data.GetString("Iptc.Application2.Caption")
	require.Error(t, err)
}

func TestXmpStripKey(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.SetXmpString("Xmp.dc.description", "123")
	require.NoError(t, err)

	err = img.XmpStripKey("Xmp.dc.description")
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	data := img.GetXmpData()

	_, err = data.GetString("Xmp.dc.description")
	require.Error(t, err)
}

func TestExifStrip(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	// add two strings to the EXIF data
	err = img.SetExifString("Exif.Photo.UserComment", "123")
	require.NoError(t, err)

	err = img.SetExifString("Exif.Photo.DateTimeOriginal", "123")
	require.NoError(t, err)

	err = img.ExifStripMetadata([]string{"Exif.Photo.UserComment"})
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	data := img.GetExifData()

	_, err = data.GetString("Exif.Photo.UserComment")
	require.NoError(t, err)

	_, err = data.GetString("Exif.Photo.DateTimeOriginal")
	require.Error(t, err)
}

func TestIptcStrip(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	// add two strings to the IPTC data
	err = img.SetIptcString("Iptc.Application2.Caption", "123")
	require.NoError(t, err)

	err = img.SetIptcString("Iptc.Application2.Keywords", "123")
	require.NoError(t, err)

	err = img.IptcStripMetadata([]string{"Iptc.Application2.Caption"})
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	data := img.GetIptcData()

	_, err = data.GetString("Iptc.Application2.Caption")
	require.NoError(t, err)

	_, err = data.GetString("Iptc.Application2.Keywords")
	require.Error(t, err)
}

func TestXmpStrip(t *testing.T) {
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	// add two strings to the XMP data
	err = img.SetXmpString("Xmp.dc.description", "123")
	require.NoError(t, err)

	err = img.SetXmpString("Xmp.dc.subject", "123")
	require.NoError(t, err)

	err = img.XmpStripMetadata([]string{"Xmp.dc.description"})
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	data := img.GetXmpData()

	_, err = data.GetString("Xmp.dc.description")
	require.NoError(t, err)

	_, err = data.GetString("Xmp.dc.subject")
	require.Error(t, err)
}

func TestStripMetadata(t *testing.T) {
	// The test for this function must need plenty of tags to ensure it won't generate an unexpected behavior
	initializeImage("testdata/pixel.jpg", t)
	img, err := goexiv.Open("testdata/pixel.jpg")
	require.NoError(t, err)

	err = img.ReadMetadata()
	require.NoError(t, err)

	err = img.StripMetadata([]string{"Exif.Image.Copyright", "Iptc.Application2.Copyright", "Xmp.iptc.CreditLine"})
	require.NoError(t, err)

	// Exif
	exifData := img.GetExifData()
	assert.Equal(t, map[string]string{
		"Exif.Image.Copyright": "©2023 John Doe, all rights reserved",
	}, exifData.AllTags())

	// IPTC
	iptcData := img.GetIptcData()
	assert.Equal(t, map[string]string{
		"Iptc.Application2.Copyright": "this is the copy, right?",
	}, iptcData.AllTags())

	// XMP
	xmpData := img.GetXmpData()
	assert.Equal(t, map[string]string{
		"Xmp.iptc.CreditLine": "John Doe",
	}, xmpData.AllTags())
}

func BenchmarkImage_GetBytes_KeepAlive(b *testing.B) {
	bytes, err := os.ReadFile("testdata/stripped_pixel.jpg")
	require.NoError(b, err)
	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			img, err := goexiv.OpenBytes(bytes)
			require.NoError(b, err)

			runtime.GC()

			require.NoError(b, img.SetExifString("Exif.Photo.UserComment", "123"))

			bytesAfter := img.GetBytes()
			assert.NotEmpty(b, bytesAfter)
			runtime.KeepAlive(img)
		}()
	}

	wg.Wait()
}

func BenchmarkImage_GetBytes_NoKeepAlive(b *testing.B) {
	bytes, err := os.ReadFile("testdata/stripped_pixel.jpg")
	require.NoError(b, err)
	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			img, err := goexiv.OpenBytes(bytes)
			require.NoError(b, err)

			require.NoError(b, img.SetExifString("Exif.Photo.UserComment", "123"))

			bytesAfter := img.GetBytes()
			assert.NotEmpty(b, bytesAfter)
		}()
	}
}

// Fills the image with metadata
func initializeImage(path string, t *testing.T) {
	img, err := goexiv.Open(path)
	require.NoError(t, err)

	img.SetIptcString("Iptc.Application2.Copyright", "this is the copy, right?")
	img.SetIptcString("Iptc.Application2.CountryName", "Lancre")
	img.SetIptcString("Iptc.Application2.DateCreated", "20121013")
	img.SetIptcString("Iptc.Application2.TimeCreated", "124932:0100")

	exifTags := map[string]string{
		"Exif.Image.Artist":                  "John Doe",
		"Exif.Image.Copyright":               "©2023 John Doe, all rights reserved",
		"Exif.Image.Make":                    "FakeMake",
		"Exif.Image.Model":                   "FakeModel",
		"Exif.Image.ResolutionUnit":          "2",
		"Exif.Image.XResolution":             "72/1",
		"Exif.Image.YCbCrPositioning":        "1",
		"Exif.Image.YResolution":             "72/1",
		"Exif.Photo.ColorSpace":              "65535",
		"Exif.Photo.ComponentsConfiguration": "1 2 3 0",
		"Exif.Photo.DateTimeDigitized":       "2013:12:08 21:06:10",
		"Exif.Photo.ExifVersion":             "48 50 51 48",
		"Exif.Photo.FlashpixVersion":         "48 49 48 48",
	}

	for k, v := range exifTags {
		err = img.SetExifString(k, v)
		require.NoError(t, err, k, v)
	}

	img.SetXmpString("Xmp.iptc.CreditLine", "John Doe")
	img.SetXmpString("Xmp.iptc.CopyrightNotice", "this is the copy, right?")
	img.SetXmpString("Xmp.iptc.JobId", "12345")
}
