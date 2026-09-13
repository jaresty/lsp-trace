package qualificationfixture

func leaf(value int) int { return value + 1 }

func left(value int) int  { return leaf(value) }
func right(value int) int { return leaf(value) }

func entry(value int) int { return left(value) + right(value) }
