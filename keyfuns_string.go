// Code generated by "stringer -type=KeyFuns"; DO NOT EDIT.

package gi

import (
	"fmt"
	"strconv"
)

const _KeyFuns_name = "KeyFunNilKeyFunMoveUpKeyFunMoveDownKeyFunMoveRightKeyFunMoveLeftKeyFunPageUpKeyFunPageDownKeyFunPageRightKeyFunPageLeftKeyFunHomeKeyFunEndKeyFunDocHomeKeyFunDocEndKeyFunWordRightKeyFunWordLeftKeyFunFocusNextKeyFunFocusPrevKeyFunEnterKeyFunAcceptKeyFunCancelSelectKeyFunSelectModeKeyFunSelectAllKeyFunAbortKeyFunEditItemKeyFunCopyKeyFunCutKeyFunPasteKeyFunBackspaceKeyFunBackspaceWordKeyFunDeleteKeyFunDeleteWordKeyFunKillKeyFunDuplicateKeyFunUndoKeyFunRedoKeyFunInsertKeyFunInsertAfterKeyFunGoGiEditorKeyFunZoomOutKeyFunZoomInKeyFunPrefsKeyFunRefreshKeyFunRecenterKeyFunCompleteKeyFunSearchKeyFunFindKeyFunJumpKeyFunHistPrevKeyFunHistNextKeyFunsN"

var _KeyFuns_index = [...]uint16{0, 9, 21, 35, 50, 64, 76, 90, 105, 119, 129, 138, 151, 163, 178, 192, 207, 222, 233, 245, 263, 279, 294, 305, 319, 329, 338, 349, 364, 383, 395, 411, 421, 436, 446, 456, 468, 485, 501, 514, 526, 537, 550, 564, 578, 590, 600, 610, 624, 638, 646}

func (i KeyFuns) String() string {
	if i < 0 || i >= KeyFuns(len(_KeyFuns_index)-1) {
		return "KeyFuns(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _KeyFuns_name[_KeyFuns_index[i]:_KeyFuns_index[i+1]]
}

func (i *KeyFuns) FromString(s string) error {
	for j := 0; j < len(_KeyFuns_index)-1; j++ {
		if s == _KeyFuns_name[_KeyFuns_index[j]:_KeyFuns_index[j+1]] {
			*i = KeyFuns(j)
			return nil
		}
	}
	return fmt.Errorf("String %v is not a valid option for type KeyFuns", s)
}
