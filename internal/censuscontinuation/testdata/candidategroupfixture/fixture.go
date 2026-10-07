package candidategroupfixture

func Entry() {
	Collect()
	Publish()
}

func Collect() {
	Normalize()
	Validate()
}

func Normalize() {
	Validate()
}

func Validate() {
	Collect()
	Publish()
}

func Publish() {
	Encode()
	Write()
}

func Encode() {
	Write()
}

func Write() {
	Publish()
}
