// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewId(t *testing.T) {
	for i := 0; i < 1000; i++ {
		id := NewId()
		require.LessOrEqual(t, len(id), 26, "ids shouldn't be longer than 26 chars")
	}
}

func TestRandomString(t *testing.T) {
	for i := 0; i < 1000; i++ {
		str := NewRandomString(i)
		require.Len(t, str, i)
		require.NotContains(t, str, "=")
	}
}

func TestGetMillisForTime(t *testing.T) {
	thisTimeMillis := int64(1471219200000)
	thisTime := time.Date(2016, time.August, 15, 0, 0, 0, 0, time.UTC)

	result := GetMillisForTime(thisTime)

	require.Equalf(t, thisTimeMillis, result, "millis are not the same: %d and %d", thisTimeMillis, result)
}

func TestGetTimeForMillis(t *testing.T) {
	thisTimeMillis := int64(1471219200000)
	thisTime := time.Date(2016, time.August, 15, 0, 0, 0, 0, time.UTC)

	result := GetTimeForMillis(thisTimeMillis)
	require.True(t, thisTime.Equal(result))
}

func TestPadDateStringZeros(t *testing.T) {
	for _, testCase := range []struct {
		Name     string
		Input    string
		Expected string
	}{
		{
			Name:     "Valid date",
			Input:    "2016-08-01",
			Expected: "2016-08-01",
		},
		{
			Name:     "Valid date but requires padding of zero",
			Input:    "2016-8-1",
			Expected: "2016-08-01",
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			assert.Equal(t, testCase.Expected, PadDateStringZeros(testCase.Input))
		})
	}
}

func TestAppErrorRender(t *testing.T) {
	t.Run("Minimal", func(t *testing.T) {
		aerr := NewAppError("here", "message", nil, "", http.StatusTeapot)
		assert.EqualError(t, aerr, "here: message")
	})

	t.Run("Without where", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "details", http.StatusTeapot)
		assert.EqualError(t, aerr, "message, details")
	})

	t.Run("Detailed", func(t *testing.T) {
		aerr := NewAppError("here", "message", nil, "details", http.StatusTeapot)
		assert.EqualError(t, aerr, "here: message, details")
	})

	t.Run("Wrapped", func(t *testing.T) {
		aerr := NewAppError("here", "message", nil, "", http.StatusTeapot).Wrap(fmt.Errorf("my error"))
		assert.EqualError(t, aerr, "here: message, my error")
	})

	t.Run("WrappedMultiple", func(t *testing.T) {
		aerr := NewAppError("here", "message", nil, "", http.StatusTeapot).Wrap(fmt.Errorf("my error (%w)", fmt.Errorf("inner error")))
		assert.EqualError(t, aerr, "here: message, my error (inner error)")
	})

	t.Run("DetailedWrappedMultiple", func(t *testing.T) {
		aerr := NewAppError("here", "message", nil, "details", http.StatusTeapot).Wrap(fmt.Errorf("my error (%w)", fmt.Errorf("inner error")))
		assert.EqualError(t, aerr, "here: message, details, my error (inner error)")
	})

	t.Run("MaxLength", func(t *testing.T) {
		str := strings.Repeat("error", 65536)
		msg := "msg"
		aerr := NewAppError("id", msg, nil, str, http.StatusTeapot).Wrap(errors.New(str))
		assert.Len(t, aerr.Error(), maxErrorLength+len(msg))
	})

	t.Run("No Translation", func(t *testing.T) {
		appErr := NewAppError("TestAppError", NoTranslation, nil, "test error", http.StatusBadRequest)
		require.Equal(t, "TestAppError: test error", appErr.Error())
	})
}

func TestAppErrorSerialize(t *testing.T) {
	t.Run("Junk", func(t *testing.T) {
		rerr := AppErrorFromJSON(strings.NewReader("<html><body>This is a broken test</body></html>"))
		require.ErrorContains(t, rerr, "failed to decode JSON payload into AppError")
		require.ErrorContains(t, rerr, "<html><body>This is a broken test</body></html>")
	})

	t.Run("Normal", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "", http.StatusTeapot)
		js := aerr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(js))
		berr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, "message", berr.Id)
		require.Empty(t, berr.DetailedError)
		require.Equal(t, http.StatusTeapot, berr.StatusCode)

		require.EqualError(t, berr, aerr.Error())
	})

	t.Run("Detailed", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "detail", http.StatusTeapot)
		js := aerr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(js))
		berr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, "message", berr.Id)
		require.Equal(t, "detail", berr.DetailedError)
		require.Equal(t, http.StatusTeapot, berr.StatusCode)

		require.EqualError(t, berr, aerr.Error())
	})

	t.Run("Wipe Detailed", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "detail", http.StatusTeapot)
		aerr.WipeDetailed()
		js := aerr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(js))
		berr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, "message", berr.Id)
		require.Equal(t, "", berr.DetailedError)
		require.Equal(t, http.StatusTeapot, berr.StatusCode)

		require.EqualError(t, berr, aerr.Error())
	})

	t.Run("Wrapped", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "", http.StatusTeapot).Wrap(errors.New("wrapped"))
		js := aerr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(js))
		berr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, "message", berr.Id)
		require.Equal(t, "wrapped", berr.DetailedError)
		require.Equal(t, http.StatusTeapot, berr.StatusCode)

		require.EqualError(t, berr, aerr.Error())
	})

	t.Run("Wipe Wrapped", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "", http.StatusTeapot).Wrap(errors.New("wrapped"))
		aerr.WipeDetailed()
		js := aerr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(js))
		berr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, "message", berr.Id)
		require.Equal(t, "", berr.DetailedError)
		require.Equal(t, http.StatusTeapot, berr.StatusCode)

		require.EqualError(t, berr, aerr.Error())
	})

	t.Run("Detailed + Wrapped", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "detail", http.StatusTeapot).Wrap(errors.New("wrapped"))
		js := aerr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(js))
		berr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, "message", berr.Id)
		require.Equal(t, "detail, wrapped", berr.DetailedError)
		require.Equal(t, http.StatusTeapot, berr.StatusCode)

		require.EqualError(t, berr, aerr.Error())
	})

	t.Run("Detailed + Wrapped", func(t *testing.T) {
		aerr := NewAppError("", "message", nil, "detail", http.StatusTeapot).Wrap(errors.New("wrapped"))
		aerr.WipeDetailed()
		js := aerr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(js))
		berr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, "message", berr.Id)
		require.Equal(t, "", berr.DetailedError)
		require.Equal(t, http.StatusTeapot, berr.StatusCode)

		require.EqualError(t, berr, aerr.Error())
	})

	t.Run("Where", func(t *testing.T) {
		appErr := NewAppError("TestAppError", "message", nil, "", http.StatusInternalServerError)
		json := appErr.ToJSON()
		err := AppErrorFromJSON(strings.NewReader(json))
		rerr, ok := err.(*AppError)
		require.True(t, ok)
		require.Equal(t, appErr.Message, rerr.Message)
	})

	t.Run("Returned http.MaxBytesError", func(t *testing.T) {
		aerr := (&http.MaxBytesError{}).Error() + "\n"

		err := AppErrorFromJSON(strings.NewReader(aerr))
		require.EqualError(t, err, "The request was too large. Consider asking your System Admin to raise the FileSettings.MaxFileSize setting.")
	})
}

func TestCopyStringMap(t *testing.T) {
	itemKey := "item1"
	originalMap := make(map[string]string)
	originalMap[itemKey] = "val1"

	copyMap := CopyStringMap(originalMap)
	copyMap[itemKey] = "changed"

	assert.Equal(t, "val1", originalMap[itemKey])
}

func TestMapJson(t *testing.T) {
	m := make(map[string]string)
	m["id"] = "test_id"
	json := MapToJSON(m)

	rm := MapFromJSON(strings.NewReader(json))

	require.Equal(t, rm["id"], "test_id", "map should be valid")

	rm2 := MapFromJSON(strings.NewReader(""))
	require.LessOrEqual(t, len(rm2), 0, "make should be invalid")
}

func TestSortedArrayFromJSON(t *testing.T) {
	t.Run("Successful parse", func(t *testing.T) {
		ids := []string{NewId(), NewId(), NewId()}
		b, _ := json.Marshal(ids)
		a, err := SortedArrayFromJSON(bytes.NewReader(b))
		require.NoError(t, err)
		require.ElementsMatch(t, ids, a)
	})

	t.Run("Empty Array", func(t *testing.T) {
		ids := []string{}
		b, _ := json.Marshal(ids)
		a, err := SortedArrayFromJSON(bytes.NewReader(b))
		require.NoError(t, err)
		require.Empty(t, a)
	})

	t.Run("Duplicate keys, returns one", func(t *testing.T) {
		var ids []string
		id := NewId()
		for i := 0; i < 10; i++ {
			ids = append(ids, id)
		}
		b, _ := json.Marshal(ids)
		a, err := SortedArrayFromJSON(bytes.NewReader(b))
		require.NoError(t, err)
		require.Len(t, a, 1)
	})
}

func TestNonSortedArrayFromJSON(t *testing.T) {
	t.Run("Successful parse", func(t *testing.T) {
		ids := []string{NewId(), NewId(), NewId()}
		b, _ := json.Marshal(ids)
		a, err := NonSortedArrayFromJSON(bytes.NewReader(b))
		require.NoError(t, err)
		require.Equal(t, ids, a)
	})

	t.Run("Empty Array", func(t *testing.T) {
		ids := []string{}
		b, _ := json.Marshal(ids)
		a, err := NonSortedArrayFromJSON(bytes.NewReader(b))
		require.NoError(t, err)
		require.Empty(t, a)
	})

	t.Run("Duplicate keys, returns one", func(t *testing.T) {
		var ids []string
		id := NewId()
		for i := 0; i <= 10; i++ {
			ids = append(ids, id)
		}
		b, _ := json.Marshal(ids)
		a, err := NonSortedArrayFromJSON(bytes.NewReader(b))
		require.NoError(t, err)
		require.Len(t, a, 1)
	})
}

func TestIsValidEmail(t *testing.T) {
	for _, testCase := range []struct {
		Input    string
		Expected bool
	}{
		{
			Input:    "corey",
			Expected: false,
		},
		{
			Input:    "corey@example.com",
			Expected: true,
		},
		{
			Input:    "corey+test@example.com",
			Expected: true,
		},
		{
			Input:    "@corey+test@example.com",
			Expected: false,
		},
		{
			Input:    "firstname.lastname@example.com",
			Expected: true,
		},
		{
			Input:    "firstname.lastname@subdomain.example.com",
			Expected: true,
		},
		{
			Input:    "123454567@domain.com",
			Expected: true,
		},
		{
			Input:    "email@domain-one.com",
			Expected: true,
		},
		{
			Input:    "email@domain.co.jp",
			Expected: true,
		},
		{
			Input:    "firstname-lastname@domain.com",
			Expected: true,
		},
		{
			Input:    "@domain.com",
			Expected: false,
		},
		{
			Input:    "Billy Bob <billy@example.com>",
			Expected: false,
		},
		{
			Input:    "email.domain.com",
			Expected: false,
		},
		{
			Input:    "email.@domain.com",
			Expected: false,
		},
		{
			Input:    "email@domain@domain.com",
			Expected: false,
		},
		{
			Input:    "(email@domain.com)",
			Expected: false,
		},
		{
			Input:    "email@汤.中国",
			Expected: true,
		},
		{
			Input:    "email1@domain.com, email2@domain.com",
			Expected: false,
		},
		{
			Input:    "\"attacker@attacker.com,admin\"@spaceship.com",
			Expected: false,
		},
		{
			Input:    "(email)@domain.com",
			Expected: false,
		},
		{
			Input:    "<email>@domain.com",
			Expected: false,
		},
		{
			Input:    "[email]@domain.com",
			Expected: false,
		},
		{
			Input:    "{email}@domain.com",
			Expected: true,
		},
		{
			Input:    "first\"name@domain.com",
			Expected: false,
		},
		{
			Input:    "first:name@domain.com",
			Expected: false,
		},
		{
			Input:    "first;name@domain.com",
			Expected: false,
		},
		{
			Input:    "first,name@domain.com",
			Expected: false,
		},
		{
			Input:    "first@name@domain.com",
			Expected: false,
		},
		{
			Input:    "john..doe@example.com",
			Expected: false,
		},
	} {
		t.Run(testCase.Input, func(t *testing.T) {
			assert.Equal(t, testCase.Expected, IsValidEmail(testCase.Input))
		})
	}
}

func TestEtag(t *testing.T) {
	etag := Etag("hello", 24)
	require.NotEqual(t, "", etag)
}

var hashtags = map[string]string{
	"#test":           "#test",
	"test":            "",
	"#test123":        "#test123",
	"#123test123":     "",
	"#test-test":      "#test-test",
	"#test?":          "#test",
	"hi #there":       "#there",
	"#bug #idea":      "#bug #idea",
	"#bug or #gif!":   "#bug #gif",
	"#hüllo":          "#hüllo",
	"#?test":          "",
	"#-test":          "",
	"#yo_yo":          "#yo_yo",
	"(#brackets)":     "#brackets",
	")#stekarb(":      "#stekarb",
	"<#less_than<":    "#less_than",
	">#greater_than>": "#greater_than",
	"-#minus-":        "#minus",
	"_#under_":        "#under",
	"+#plus+":         "#plus",
	"=#equals=":       "#equals",
	"%#pct%":          "#pct",
	"&#and&":          "#and",
	"^#hat^":          "#hat",
	"##brown#":        "#brown",
	"*#star*":         "#star",
	"|#pipe|":         "#pipe",
	":#colon:":        "#colon",
	";#semi;":         "#semi",
	"#Mötley;":        "#Mötley",
	".#period.":       "#period",
	"¿#upside¿":       "#upside",
	"\"#quote\"":      "#quote",
	"/#slash/":        "#slash",
	"\\#backslash\\":  "#backslash",
	"#a":              "",
	"#1":              "",
	"foo#bar":         "",
}

func TestStringArray_Equal(t *testing.T) {
	for name, tc := range map[string]struct {
		Array1   StringArray
		Array2   StringArray
		Expected bool
	}{
		"Empty": {
			nil,
			nil,
			true,
		},
		"EqualLength_EqualValue": {
			StringArray{"123"},
			StringArray{"123"},
			true,
		},
		"DifferentLength": {
			StringArray{"123"},
			StringArray{"123", "abc"},
			false,
		},
		"DifferentValues_EqualLength": {
			StringArray{"123"},
			StringArray{"abc"},
			false,
		},
		"EqualLength_EqualValues": {
			StringArray{"123", "abc"},
			StringArray{"123", "abc"},
			true,
		},
		"EqualLength_EqualValues_DifferentOrder": {
			StringArray{"abc", "123"},
			StringArray{"123", "abc"},
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.Expected, tc.Array1.Equals(tc.Array2))
		})
	}
}

func TestParseHashtags(t *testing.T) {
	for input, output := range hashtags {
		o, _ := ParseHashtags(input)
		require.Equal(t, o, output, "failed to parse hashtags from input="+input+" expected="+output+" actual="+o)
	}
}

func TestIsValidAlphaNum(t *testing.T) {
	cases := []struct {
		Input  string
		Result bool
	}{
		{
			Input:  "test",
			Result: true,
		},
		{
			Input:  "test-name",
			Result: true,
		},
		{
			Input:  "test--name",
			Result: true,
		},
		{
			Input:  "test__name",
			Result: true,
		},
		{
			Input:  "-",
			Result: false,
		},
		{
			Input:  "__",
			Result: false,
		},
		{
			Input:  "test-",
			Result: false,
		},
		{
			Input:  "test--",
			Result: false,
		},
		{
			Input:  "test__",
			Result: false,
		},
		{
			Input:  "test:name",
			Result: false,
		},
	}

	for _, tc := range cases {
		actual := isValidAlphaNum(tc.Input)
		require.Equalf(t, actual, tc.Result, "case: %v\tshould returned: %#v", tc, tc.Result)
	}
}

func TestGetServerIPAddress(t *testing.T) {
	require.NotEmpty(t, GetServerIPAddress(""), "Should find local ip address")
}

func TestIsValidAlphaNumHyphenUnderscore(t *testing.T) {
	casesWithFormat := []struct {
		Input  string
		Result bool
	}{
		{
			Input:  "test",
			Result: true,
		},
		{
			Input:  "test-name",
			Result: true,
		},
		{
			Input:  "test--name",
			Result: true,
		},
		{
			Input:  "test__name",
			Result: true,
		},
		{
			Input:  "test_name",
			Result: true,
		},
		{
			Input:  "test_-name",
			Result: true,
		},
		{
			Input:  "-",
			Result: false,
		},
		{
			Input:  "__",
			Result: false,
		},
		{
			Input:  "test-",
			Result: false,
		},
		{
			Input:  "test--",
			Result: false,
		},
		{
			Input:  "test__",
			Result: false,
		},
		{
			Input:  "test:name",
			Result: false,
		},
	}

	for _, tc := range casesWithFormat {
		actual := IsValidAlphaNumHyphenUnderscore(tc.Input, true)
		require.Equalf(t, actual, tc.Result, "case: %v\tshould returned: %#v", tc, tc.Result)
	}

	casesWithoutFormat := []struct {
		Input  string
		Result bool
	}{
		{
			Input:  "test",
			Result: true,
		},
		{
			Input:  "test-name",
			Result: true,
		},
		{
			Input:  "test--name",
			Result: true,
		},
		{
			Input:  "test__name",
			Result: true,
		},
		{
			Input:  "test_name",
			Result: true,
		},
		{
			Input:  "test_-name",
			Result: true,
		},
		{
			Input:  "-",
			Result: true,
		},
		{
			Input:  "_",
			Result: true,
		},
		{
			Input:  "test-",
			Result: true,
		},
		{
			Input:  "test--",
			Result: true,
		},
		{
			Input:  "test__",
			Result: true,
		},
		{
			Input:  ".",
			Result: false,
		},

		{
			Input:  "test,",
			Result: false,
		},
		{
			Input:  "test:name",
			Result: false,
		},
	}

	for _, tc := range casesWithoutFormat {
		actual := IsValidAlphaNumHyphenUnderscore(tc.Input, false)
		require.Equalf(t, actual, tc.Result, "case: '%v'\tshould returned: %#v", tc.Input, tc.Result)
	}
}

func TestIsValidAlphaNumHyphenUnderscorePlus(t *testing.T) {
	cases := []struct {
		Input  string
		Result bool
	}{
		{
			Input:  "test",
			Result: true,
		},
		{
			Input:  "test+name",
			Result: true,
		},
		{
			Input:  "test+-name",
			Result: true,
		},
		{
			Input:  "test_+name",
			Result: true,
		},
		{
			Input:  "test++name",
			Result: true,
		},
		{
			Input:  "test_-name",
			Result: true,
		},
		{
			Input:  "-",
			Result: true,
		},
		{
			Input:  "_",
			Result: true,
		},
		{
			Input:  "+",
			Result: true,
		},
		{
			Input:  "test+",
			Result: true,
		},
		{
			Input:  "test++",
			Result: true,
		},
		{
			Input:  "test--",
			Result: true,
		},
		{
			Input:  "test__",
			Result: true,
		},
		{
			Input:  ".",
			Result: false,
		},

		{
			Input:  "test,",
			Result: false,
		},
		{
			Input:  "test:name",
			Result: false,
		},
	}

	for _, tc := range cases {
		actual := IsValidAlphaNumHyphenUnderscorePlus(tc.Input)
		require.Equalf(t, actual, tc.Result, "case: '%v'\tshould returned: %#v", tc.Input, tc.Result)
	}
}

func TestIsValidId(t *testing.T) {
	cases := []struct {
		Input  string
		Result bool
	}{
		{
			Input:  NewId(),
			Result: true,
		},
		{
			Input:  "",
			Result: false,
		},
		{
			Input:  "junk",
			Result: false,
		},
		{
			Input:  "qwertyuiop1234567890asdfg{",
			Result: false,
		},
		{
			Input:  NewId() + "}",
			Result: false,
		},
	}

	for _, tc := range cases {
		actual := IsValidId(tc.Input)
		require.Equalf(t, actual, tc.Result, "case: %v\tshould returned: %#v", tc, tc.Result)
	}
}

func TestNowhereNil(t *testing.T) {
	t.Parallel()

	var nilStringPtr *string
	var nonNilStringPtr = new(string)
	var nilSlice []string
	var nilStruct *struct{}
	var nilMap map[bool]bool

	var nowhereNilStruct = struct {
		X *string
		Y *string
	}{
		nonNilStringPtr,
		nonNilStringPtr,
	}
	var somewhereNilStruct = struct {
		X *string
		Y *string
	}{
		nonNilStringPtr,
		nilStringPtr,
	}

	var privateSomewhereNilStruct = struct {
		X *string
		y *string
	}{
		nonNilStringPtr,
		nilStringPtr,
	}

	testCases := []struct {
		Description string
		Value       any
		Expected    bool
	}{
		{
			"nil",
			nil,
			false,
		},
		{
			"empty string",
			"",
			true,
		},
		{
			"non-empty string",
			"not empty!",
			true,
		},
		{
			"nil string pointer",
			nilStringPtr,
			false,
		},
		{
			"non-nil string pointer",
			nonNilStringPtr,
			true,
		},
		{
			"0",
			0,
			true,
		},
		{
			"1",
			1,
			true,
		},
		{
			"0 (int64)",
			int64(0),
			true,
		},
		{
			"1 (int64)",
			int64(1),
			true,
		},
		{
			"true",
			true,
			true,
		},
		{
			"false",
			false,
			true,
		},
		{
			"nil slice",
			nilSlice,
			// A nil slice is observably the same as an empty slice, so allow it.
			true,
		},
		{
			"empty slice",
			[]string{},
			true,
		},
		{
			"slice containing nils",
			[]*string{nil, nil},
			true,
		},
		{
			"nil map",
			nilMap,
			false,
		},
		{
			"non-nil map",
			make(map[bool]bool),
			true,
		},
		{
			"non-nil map containing nil",
			map[bool]*string{true: nilStringPtr, false: nonNilStringPtr},
			// Map values are not checked
			true,
		},
		{
			"nil struct",
			nilStruct,
			false,
		},
		{
			"empty struct",
			struct{}{},
			true,
		},
		{
			"struct containing no nil",
			nowhereNilStruct,
			true,
		},
		{
			"struct containing nil",
			somewhereNilStruct,
			false,
		},
		{
			"struct pointer containing no nil",
			&nowhereNilStruct,
			true,
		},
		{
			"struct pointer containing nil",
			&somewhereNilStruct,
			false,
		},
		{
			"struct containing private nil",
			privateSomewhereNilStruct,
			true,
		},
		{
			"struct pointer containing private nil",
			&privateSomewhereNilStruct,
			true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic: %v", r)
				}
			}()

			t.Parallel()
			require.Equal(t, testCase.Expected, checkNowhereNil(t, "value", testCase.Value))
		})
	}
}

// checkNowhereNil checks that the given interface value is not nil, and if a struct, that all of
// its public fields are also nowhere nil
func checkNowhereNil(t *testing.T, name string, value any) bool {
	if value == nil {
		return false
	}

	v := reflect.ValueOf(value)
	switch v.Type().Kind() {
	case reflect.Ptr:
		// Ignoring these 2 settings.
		// TODO: remove them completely in v8.0.
		if name == "config.BleveSettings.BulkIndexingTimeWindowSeconds" ||
			name == "config.ElasticsearchSettings.BulkIndexingTimeWindowSeconds" {
			return true
		}

		if v.IsNil() {
			t.Logf("%s was nil", name)
			return false
		}

		return checkNowhereNil(t, fmt.Sprintf("(*%s)", name), v.Elem().Interface())

	case reflect.Map:
		if v.IsNil() {
			t.Logf("%s was nil", name)
			return false
		}

		// Don't check map values
		return true

	case reflect.Struct:
		nowhereNil := true
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			// Ignore unexported fields
			if v.Type().Field(i).PkgPath != "" {
				continue
			}

			nowhereNil = nowhereNil && checkNowhereNil(t, fmt.Sprintf("%s.%s", name, v.Type().Field(i).Name), f.Interface())
		}

		return nowhereNil

	case reflect.Array:
		fallthrough
	case reflect.Chan:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Interface:
		fallthrough
	case reflect.UnsafePointer:
		t.Logf("unhandled field %s, type: %s", name, v.Type().Kind())
		return false

	default:
		return true
	}
}

func TestSanitizeUnicode(t *testing.T) {
	buf := bytes.Buffer{}
	buf.WriteString("Hello")
	buf.WriteRune(0x1d173)
	buf.WriteRune(0x1d17a)
	buf.WriteString(" there.")

	musicArg := buf.String()
	musicWant := "Hello there."

	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "empty string", arg: "", want: ""},
		{name: "ascii only", arg: "Hello There", want: "Hello There"},
		{name: "allowed unicode", arg: "Ādam likes Iñtërnâtiônàližætiøn", want: "Ādam likes Iñtërnâtiônàližætiøn"},
		{name: "allowed unicode escaped", arg: "\u00eaI like hats\u00e2", want: "êI like hatsâ"},
		{name: "blocklist char, don't reverse string", arg: "\u202E2resu", want: "2resu"},
		{name: "blocklist chars, scoping musical notation", arg: musicArg, want: musicWant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeUnicode(tt.arg)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidChannelIdentifier(t *testing.T) {
	cases := []struct {
		Description string
		Input       string
		Expected    bool
	}{
		{
			Description: "less than min length",
			Input:       "",
			Expected:    false,
		},
		{
			Description: "single alphabetical char",
			Input:       "a",
			Expected:    true,
		},
		{
			Description: "single underscore",
			Input:       "_",
			Expected:    false,
		},
		{
			Description: "single hyphen",
			Input:       "-",
			Expected:    false,
		},
		{
			Description: "empty string",
			Input:       " ",
			Expected:    false,
		},
		{
			Description: "multiple with hyphen",
			Input:       "a-a",
			Expected:    true,
		},
		{
			Description: "multiple with hyphen",
			Input:       "a_a",
			Expected:    true,
		},
	}

	for _, tc := range cases {
		actual := IsValidChannelIdentifier(tc.Input)
		require.Equalf(t, actual, tc.Expected, "case: '%v'\tshould returned: %#v", tc.Input, tc.Expected)
	}
}

func TestIsValidHTTPURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Description string
		Value       string
		Expected    bool
	}{
		{
			"empty url",
			"",
			false,
		},
		{
			"bad url",
			"bad url",
			false,
		},
		{
			"relative url",
			"/api/test",
			false,
		},
		{
			"relative url ending with slash",
			"/some/url/",
			false,
		},
		{
			"url with invalid scheme",
			"http-bad://mattermost.com",
			false,
		},
		{
			"url with just http",
			"http://",
			false,
		},
		{
			"url with just https",
			"https://",
			false,
		},
		{
			"url with extra slashes",
			"https:///mattermost.com",
			false,
		},
		{
			"correct url with http scheme",
			"http://mattermost.com",
			true,
		},
		{
			"correct url with https scheme",
			"https://mattermost.com/api/test",
			true,
		},
		{
			"correct url with port",
			"https://localhost:8080/test",
			true,
		},
		{
			"correct url without scheme",
			"mattermost.com/some/url/",
			false,
		},
		{
			"correct url with extra slashes",
			"https://mattermost.com/some//url",
			true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic: %v", r)
				}
			}()

			t.Parallel()
			require.Equal(t, testCase.Expected, IsValidHTTPURL(testCase.Value))
		})
	}
}

func TestRemoveDuplicateStrings(t *testing.T) {
	cases := []struct {
		Input  []string
		Result []string
	}{
		{
			Input:  []string{"1", "2", "3", "3", "3"},
			Result: []string{"1", "2", "3"},
		},
		{
			Input:  []string{"1", "2", "3", "4", "5"},
			Result: []string{"1", "2", "3", "4", "5"},
		},
		{
			Input:  []string{"1", "1", "1", "3", "3"},
			Result: []string{"1", "3"},
		},
		{
			Input:  []string{"1", "1", "1", "1", "1"},
			Result: []string{"1"},
		},
		{
			Input:  []string{},
			Result: []string{},
		},
	}

	for _, tc := range cases {
		actual := RemoveDuplicateStrings(tc.Input)
		require.Equalf(t, actual, tc.Result, "case: %v\tshould returned: %#v", tc, tc.Result)
	}
}

func TestStructFromJSONLimited(t *testing.T) {
	t.Run("successfully parses basic struct", func(t *testing.T) {
		type TestStruct struct {
			StringField string
			IntField    int
			FloatField  float32
			BoolField   bool
		}

		testStruct := TestStruct{
			StringField: "string",
			IntField:    2,
			FloatField:  3.1415,
			BoolField:   true,
		}
		testStructBytes, err := json.Marshal(testStruct)
		require.NoError(t, err)

		b := &TestStruct{}
		err = StructFromJSONLimited(bytes.NewReader(testStructBytes), b)
		require.NoError(t, err)

		require.Equal(t, b.StringField, "string")
		require.Equal(t, b.IntField, 2)
		require.Equal(t, b.FloatField, float32(3.1415))
		require.Equal(t, b.BoolField, true)
	})

	t.Run("successfully parses nested struct", func(t *testing.T) {
		type TestStruct struct {
			StringField string
			IntField    int
			FloatField  float32
			BoolField   bool
		}

		type NestedStruct struct {
			FieldA TestStruct
			FieldB TestStruct
			FieldC []int
		}

		testStructA := TestStruct{
			StringField: "string A",
			IntField:    2,
			FloatField:  3.1415,
			BoolField:   true,
		}

		testStructB := TestStruct{
			StringField: "string B",
			IntField:    3,
			FloatField:  100,
			BoolField:   false,
		}

		nestedStruct := NestedStruct{
			FieldA: testStructA,
			FieldB: testStructB,
			FieldC: []int{5, 9, 1, 5, 7},
		}

		nestedStructBytes, err := json.Marshal(nestedStruct)
		require.NoError(t, err)

		b := &NestedStruct{}
		err = StructFromJSONLimited(bytes.NewReader(nestedStructBytes), b)
		require.NoError(t, err)

		require.Equal(t, b.FieldA.StringField, "string A")
		require.Equal(t, b.FieldA.IntField, 2)
		require.Equal(t, b.FieldA.FloatField, float32(3.1415))
		require.Equal(t, b.FieldA.BoolField, true)

		require.Equal(t, b.FieldB.StringField, "string B")
		require.Equal(t, b.FieldB.IntField, 3)
		require.Equal(t, b.FieldB.FloatField, float32(100))
		require.Equal(t, b.FieldB.BoolField, false)

		require.Equal(t, b.FieldC, []int{5, 9, 1, 5, 7})
	})

	t.Run("handles empty structs", func(t *testing.T) {
		type TestStruct struct{}

		testStruct := TestStruct{}
		testStructBytes, err := json.Marshal(testStruct)
		require.NoError(t, err)

		b := &TestStruct{}
		err = StructFromJSONLimited(bytes.NewReader(testStructBytes), b)
		require.NoError(t, err)
	})
}

func OriginParseHashtags(text string) (string, string) {
	words := strings.Fields(text)

	hashtagString := ""
	plainString := ""
	for _, word := range words {
		// trim off surrounding punctuation
		word = puncStart.ReplaceAllString(word, "")
		word = puncEnd.ReplaceAllString(word, "")

		// and remove extra pound #s
		word = hashtagStart.ReplaceAllString(word, "#")

		if validHashtag.MatchString(word) {
			hashtagString += " " + word
		} else {
			plainString += " " + word
		}
	}

	if len(hashtagString) > 1000 {
		hashtagString = hashtagString[:999]
		lastSpace := strings.LastIndex(hashtagString, " ")
		if lastSpace > -1 {
			hashtagString = hashtagString[:lastSpace]
		} else {
			hashtagString = ""
		}
	}

	return strings.TrimSpace(hashtagString), strings.TrimSpace(plainString)
}

func OptimizedParseHashTags(text string) (string, string) {
	words := strings.Fields(text)

	// Use strings.Builder for better performance
	var hashtagBuilder strings.Builder
	var plainBuilder strings.Builder

	for _, word := range words {
		// trim off surrounding punctuation
		word = puncStart.ReplaceAllString(word, "")
		word = puncEnd.ReplaceAllString(word, "")

		// and remove extra pound #s
		word = hashtagStart.ReplaceAllString(word, "#")

		if validHashtag.MatchString(word) {
			hashtagBuilder.WriteString(" ")
			hashtagBuilder.WriteString(word)
		} else {
			plainBuilder.WriteString(" ")
			plainBuilder.WriteString(word)
		}
	}

	// Get hashtag string for length limit processing
	hashtagString := hashtagBuilder.String()

	// Limit hashtag string length to 1000 chars
	// If exceeded, trim to last space or empty
	if len(hashtagString) > 1000 {
		hashtagString = hashtagString[:999]
		lastSpace := strings.LastIndex(hashtagString, " ")
		if lastSpace > -1 {
			hashtagString = hashtagString[:lastSpace]
		} else {
			hashtagString = ""
		}
	}

	return strings.TrimSpace(hashtagString), strings.TrimSpace(plainBuilder.String())
}

func OptimizedWithGrowParseHashTags(text string) (string, string) {
	words := bytes.Fields([]byte(text))

	estimatedCap := min(len(text)/2, 1000)

	// Use strings.Builder for better performance
	var hashtagBuilder strings.Builder
	var plainBuilder strings.Builder

	hashtagBuilder.Grow(estimatedCap)
	plainBuilder.Grow(len(text) - estimatedCap)

	template := []byte("$value")
	for _, word := range words {
		wordTrimmed := []byte{}
		// trim off surrounding punctuation
		for _, punctuationMatch := range punc.FindAllSubmatchIndex(word, -1) {
			wordTrimmed = punc.Expand(wordTrimmed, template, word, punctuationMatch)
		}

		// and remove extra pound #s
		word = hashtagStart.ReplaceAll(word, []byte("#"))

		if validHashtag.Match(word) {
			hashtagBuilder.WriteString(" ")
			hashtagBuilder.Write(word)
		} else {
			plainBuilder.WriteString(" ")
			plainBuilder.Write(word)
		}
	}

	// Get hashtag string for length limit processing
	hashtagString := hashtagBuilder.String()

	// Limit hashtag string length to 1000 chars
	// If exceeded, trim to last space or empty
	if len(hashtagString) > 1000 {
		hashtagString = hashtagString[:999]
		lastSpace := strings.LastIndex(hashtagString, " ")
		if lastSpace > -1 {
			hashtagString = hashtagString[:lastSpace]
		} else {
			hashtagString = ""
		}
	}

	return strings.TrimSpace(hashtagString), strings.TrimSpace(plainBuilder.String())
}

// Test cases with different sizes and characteristics
var benchmarkCases = []struct {
	name string
	text string
}{
	{
		name: "Small_NoHashtags",
		text: "Just a simple text without any hashtags",
	},
	{
		name: "Small_WithHashtags",
		text: "Post with #simple #hashtags here",
	},
	{
		name: "Medium_MixedContent",
		text: generateTestString(100, 0.3), // 30% hashtags
	},
	{
		name: "Medium_PureText",
		text: generateTestString(100, 0.0), // 0% hashtags
	},
	{
		name: "Large_MixedContent",
		text: generateTestString(1000, 0.3), // 30% hashtags
	},
	{
		name: "Large_PureText",
		text: generateTestString(1000, 0), // 0% hashtags
	},
	{
		name: "VeryLarge_MixedContent",
		text: generateTestString(5000, 0.3), // 30% hashtags
	},
	{
		name: "VeryLarge_PureText",
		text: generateTestString(5000, 0.0), // 0% hashtags
	},
}

// Helper function to generate test strings with specified size and hashtag ratio
func generateTestString(wordCount int, hashtagRatio float64) string {
	var builder strings.Builder
	hashtagWords := []string{"#golang", "#programming", "#testing", "#benchmark", "#performance"}
	normalWords := []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog"}

	for i := 0; i < wordCount; i++ {
		if float64(i)/float64(wordCount) < hashtagRatio {
			// Add hashtag word
			builder.WriteString(hashtagWords[i%len(hashtagWords)])
		} else {
			// Add normal word
			builder.WriteString(normalWords[i%len(normalWords)])
		}
		builder.WriteString(" ")
	}
	return builder.String()
}

// Benchmark original implementation
func BenchmarkOriginParseHashtags(b *testing.B) {
	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs() // Report memory allocations
			b.ResetTimer()   // Reset timer before the actual benchmark
			for i := 0; i < b.N; i++ {
				OriginParseHashtags(bc.text)
			}
		})
	}
}

// Benchmark optimized implementation
func BenchmarkOptimizedParseHashTags(b *testing.B) {
	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs() // Report memory allocations
			b.ResetTimer()   // Reset timer before the actual benchmark
			for i := 0; i < b.N; i++ {
				OptimizedParseHashTags(bc.text)
			}
		})
	}
}

// Validate that both implementations produce the same results
func TestImplementationsEquivalence(t *testing.T) {
	for _, tc := range benchmarkCases {
		t.Run(tc.name, func(t *testing.T) {
			originalHashtags, originalPlain := OriginParseHashtags(tc.text)
			optimizedHashtags, optimizedPlain := OptimizedParseHashTags(tc.text)

			if originalHashtags != optimizedHashtags {
				t.Errorf("Hashtags mismatch for %s:\nOriginal:  %s\nOptimized: %s",
					tc.name, originalHashtags, optimizedHashtags)
			}
			if originalPlain != optimizedPlain {
				t.Errorf("Plain text mismatch for %s:\nOriginal:  %s\nOptimized: %s",
					tc.name, originalPlain, optimizedPlain)
			}
		})
	}
}
