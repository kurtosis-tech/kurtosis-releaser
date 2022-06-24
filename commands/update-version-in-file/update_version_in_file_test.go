package updateversioninfile

import (
	"testing"
)

func TestDoBreakingChangesExist(t *testing.T) {
	noVersion :=
		`# TBD
* Something
* Something else`

	onlyOneVersion :=
		`#TBD
* Something

#0.1.0
* Something`

	onlyOneVersionWithSpaces :=
		`# TBD
* Something

# 0.1.0
* Something`

	onlyOneVersionTwoHashBreakingChanges :=
		`#TBD
* Something

##Breaking Changes
* Something else

#0.1.0
* Something`

	onlyOneVersionThreeHashBreakingChanges :=
		`#TBD
* Something

###Breaking Changes
* Something else

#0.1.0
* Something`

	onlyOneVersionFourHashBreakingChanges :=
		`#TBD
* Something

####Breaking Changes
* Something else

#0.1.0
* Something`

	multipleVersions :=
		`#TBD
* Something

#0.1.1
* Something else

#0.1.0
* Something`

	multipleVersionsBreakingChanges :=
		`#TBD
* Something

### Breaking Changes
* Something

#0.1.1
* Something else

#0.1.0
* Something`

	lowercaseBreakingChanges :=
		`# TBD
### breaking changes
* Some breaks

# 0.1.0
* Something`

	shouldHaveBreakingChanges := []string{onlyOneVersionTwoHashBreakingChanges, onlyOneVersionThreeHashBreakingChanges, onlyOneVersionFourHashBreakingChanges, multipleVersionsBreakingChanges, lowercaseBreakingChanges}
	shouldNotHaveBreakingChanges := []string{noVersion, onlyOneVersion, onlyOneVersionWithSpaces, multipleVersions}

	testBreakingChangesExists(t, shouldHaveBreakingChanges, shouldNotHaveBreakingChanges)
}
