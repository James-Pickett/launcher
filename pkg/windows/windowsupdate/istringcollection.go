package windowsupdate

import (
	"fmt"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/kolide/launcher/pkg/windows/oleconv"
)

// iStringCollectionToStringArrayErr takes a IDispatch to a
// stringcollection, and returns the array of strings
// https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-istringcollection
func iStringCollectionToStringArrayErr(disp *ole.IDispatch, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}

	if disp == nil {
		return nil, nil
	}

	count, err := oleconv.ToInt32Err(oleutil.GetProperty(disp, "Count"))
	if err != nil {
		return nil, fmt.Errorf("getting property Count as int32: %w", err)
	}

	stringCollection := make([]string, count)

	for i := 0; i < int(count); i++ {
		str, err := oleconv.ToStringErr(oleutil.GetProperty(disp, "Item", i))
		if err != nil {
			return nil, fmt.Errorf("getting property Item at index %d of %d as string: %w", i, count, err)
		}

		stringCollection[i] = str
	}
	return stringCollection, nil
}
