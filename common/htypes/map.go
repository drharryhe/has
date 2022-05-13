package htypes

type Map map[string]interface{}

func (this *Map) To() map[string]interface{} {
	return map[string]interface{}(*this)
}

func (this *Map) From(v map[string]interface{}) {
	*this = v
}
