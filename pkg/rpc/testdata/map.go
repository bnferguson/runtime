package maptype

import (
	"encoding/json"
	"maps"

	"github.com/fxamacker/cbor/v2"
)

type heroData struct {
	Age    *int32             `cbor:"0,keyasint,omitempty" json:"age,omitempty"`
	Name   *string            `cbor:"1,keyasint,omitempty" json:"name,omitempty"`
	Traits *map[string]string `cbor:"2,keyasint,omitempty" json:"traits,omitempty"`
}

type Hero struct {
	data heroData
}

func (v *Hero) HasAge() bool {
	return v.data.Age != nil
}

func (v *Hero) Age() int32 {
	if v.data.Age == nil {
		return 0
	}
	return *v.data.Age
}

func (v *Hero) SetAge(age int32) {
	v.data.Age = &age
}

func (v *Hero) HasName() bool {
	return v.data.Name != nil
}

func (v *Hero) Name() string {
	if v.data.Name == nil {
		return ""
	}
	return *v.data.Name
}

func (v *Hero) SetName(name string) {
	v.data.Name = &name
}

func (v *Hero) HasTraits() bool {
	return v.data.Traits != nil
}

func (v *Hero) Traits() map[string]string {
	if v.data.Traits == nil {
		return nil
	}
	return *v.data.Traits
}

func (v *Hero) SetTraits(traits map[string]string) {
	x := maps.Clone(traits)
	v.data.Traits = &x
}

func (v *Hero) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *Hero) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *Hero) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *Hero) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}
