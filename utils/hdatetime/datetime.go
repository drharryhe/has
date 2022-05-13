package hdatetime

import "time"

func Now() string {
	return time.Now().Local().Format("2006-01-02 15:04:05")
}

func Today() string {
	return time.Now().Local().Format("2006-01-02")
}

func Yesterday() string {
	loc, _ := time.LoadLocation("Local")
	nTime := time.Now().In(loc)
	yesTime := nTime.AddDate(0, 0, -1)
	return yesTime.Format("2006-01-02")
}

func Tomorrow() string {
	loc, _ := time.LoadLocation("Local")
	nTime := time.Now().In(loc)
	t := nTime.AddDate(0, 0, 1)
	return t.Format("2006-01-02")
}

func AfterDays(day string, days int) string {
	loc, _ := time.LoadLocation("Local")
	d, _ := time.ParseInLocation("2006-01-02", day, loc)
	return d.AddDate(0, 0, days).Format("2006-01-02")
}

func BeforeDays(from string, days int) string {
	if t, err := time.Parse("2006-01-02", from); err != nil {
		return ""
	} else {
		day := t.AddDate(0, 0, -days)
		return day.Format("2006-01-02")
	}
}

func TimeFromString(t string) time.Time {
	stamp, _ := time.ParseInLocation("2006-01-02 15:04:05", t, time.Local) //使用parseInLocation将字符串格式化返回本地时区时间
	return stamp
}

func DateFromString(t string) time.Time {
	stamp, _ := time.ParseInLocation("2006-01-02", t, time.Local) //使用parseInLocation将字符串格式化返回本地时区时间
	return stamp
}
