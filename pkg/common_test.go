package common

import (
	"testing"
)

type recordPrefixMatchesNodePrefixesTest struct {
	recordName string
	nodeNames  []string
	expected   bool
}

var recordPrefixMatchesNodePrefixesTestCases = []recordPrefixMatchesNodePrefixesTest{
	{
		"sfu-123.gather.town",
		[]string{"sfu-123-313"},
		true,
	},
	{
		"sfu-123.gather.town",
		[]string{"sfu-abc", "ip-1-2-3"},
		true,
	},
	{
		"sfu-123.gather.town",
		[]string{"xyz-"},
		false,
	},
}

func TestRecordPrefixMatchesNodePrefixes(t *testing.T) {
	for _, test := range recordPrefixMatchesNodePrefixesTestCases {
		if output := RecordPrefixMatchesNodePrefixes(test.recordName, test.nodeNames); output != test.expected {
			t.Errorf("Output for test %v is not equal to expected %v", test, test.expected)
		}
	}
}
