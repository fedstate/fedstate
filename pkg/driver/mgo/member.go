package mgo

type memberUtil int

var StaticMemberUtil = new(memberUtil)

func (s *memberUtil) MembersAddrs(rsMembers []Member) []string {
	var addrs []string

	for _, v := range rsMembers {
		addrs = append(addrs, v.Host)
	}
	return addrs
}

func (s *memberUtil) AddMembers(src, add []Member) ([]Member, bool) {
	set := make(map[string]Member)
	maxMemberId := 0
	for _, m := range src {
		set[m.Host] = m
		if m.ID > maxMemberId {
			maxMemberId = m.ID
		}
	}

	var changed bool
	i := 1
	for _, m := range add {
		if _, ok := set[m.Host]; !ok {
			changed = true
			// 添加 member id 取最大的member id+1
			m.ID = maxMemberId + i
			set[m.Host] = m
			i++
		}
	}
	if changed {
		var r []Member
		for _, m := range set {
			r = append(r, m)
		}
		return r, true
	}
	return src, false
}

// 判断Member是否已存在于配置中
func (s *memberUtil) MemberExist(src, add []Member) (bool, []Member) {
	var addMember []Member
	if len(add) == 0 {
		// 不处理添加节点，表示已经存在
		return true, addMember
	}

	set := make(map[string]string)
	maxMemberId := 0
	for _, m := range src {
		set[m.Host] = ""
		if m.ID > maxMemberId {
			maxMemberId = m.ID
		}
	}
	i := 1
	for _, m := range add {
		if _, ok := set[m.Host]; !ok {
			// 添加 member id 取最大的member id+1
			m.ID = maxMemberId + i
			addMember = append(addMember, m)
			i++
		}
	}
	if len(addMember) > 0 {
		return false, addMember
	}

	// should be unreachable
	return true, addMember
}

// 移除Member
func (s *memberUtil) RemoveMembers(src, del []Member) ([]Member, bool) {
	set := make(map[string]Member)
	// 判断待移除的member是否在member列表里存在，如果不存在则不需要处理
	var changed bool
	for _, m := range src {
		set[m.Host] = m
	}
	for _, m := range del {
		if _, ok := set[m.Host]; ok {
			changed = true
			delete(set, m.Host)
		}
	}
	if changed {
		var r []Member
		for _, m := range set {
			r = append(r, m)
		}
		return r, true
	}
	return src, false
}
