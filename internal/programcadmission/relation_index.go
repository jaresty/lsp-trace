package programcadmission

import (
	"errors"
	"sort"
)

var errRelationIndexIntegrity = errors.New("invalid relation index input integrity")

type relationIndex struct {
	occurrences []TypedOccurrence
	outgoing    map[string][]TypedOccurrence
	incoming    map[string][]TypedOccurrence
}

func buildRelationIndex(in TypedInput) (relationIndex, error) {
	if err := validateTypedInputIntegrity(in); err != nil {
		return relationIndex{}, errors.Join(errRelationIndexIntegrity, err)
	}

	index := relationIndex{
		occurrences: append([]TypedOccurrence(nil), in.Occurrences...),
		outgoing:    make(map[string][]TypedOccurrence),
		incoming:    make(map[string][]TypedOccurrence),
	}
	for _, occurrence := range index.occurrences {
		index.outgoing[occurrence.From] = append(index.outgoing[occurrence.From], occurrence)
		index.incoming[occurrence.To] = append(index.incoming[occurrence.To], occurrence)
	}
	sortTypedOccurrences(index.occurrences)
	for endpoint := range index.outgoing {
		sortTypedOccurrences(index.outgoing[endpoint])
	}
	for endpoint := range index.incoming {
		sortTypedOccurrences(index.incoming[endpoint])
	}
	return index, nil
}

func (index relationIndex) allOccurrences() []TypedOccurrence {
	return append([]TypedOccurrence(nil), index.occurrences...)
}

func (index relationIndex) outgoingOccurrences(endpoint string) []TypedOccurrence {
	return append([]TypedOccurrence(nil), index.outgoing[endpoint]...)
}

func (index relationIndex) incomingOccurrences(endpoint string) []TypedOccurrence {
	return append([]TypedOccurrence(nil), index.incoming[endpoint]...)
}

func sortTypedOccurrences(occurrences []TypedOccurrence) {
	sort.SliceStable(occurrences, func(i, j int) bool {
		return occurrences[i].Identity < occurrences[j].Identity
	})
}
