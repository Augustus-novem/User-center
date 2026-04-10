package biztime

import (
	"fmt"
	"sync"
	"time"
)

const businessTZ = "Asia/Shanghai"

var (
	loc     *time.Location
	locOnce sync.Once
)

func Location() *time.Location {
	locOnce.Do(func() {
		loaded, err := time.LoadLocation(businessTZ)
		if err != nil {
			loaded = time.FixedZone("CST", 8*60*60)
		}
		loc = loaded
	})
	return loc
}

func StartOfBizDay(ts int64) time.Time {
	t := ToTime(ts)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Location())
}

func StartOfDay(t time.Time) time.Time {
	t = t.In(Location())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Location())
}

func NextDayStart(t time.Time) time.Time {
	return StartOfDay(t).AddDate(0, 0, 1)
}

func GetYearMonth(t time.Time) (int, time.Month) {
	return t.In(Location()).Year(), t.In(Location()).Month()
}

func MonthStart(t time.Time) time.Time {
	year, month := GetYearMonth(t)
	return time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
}

func NextMonthStart(t time.Time) time.Time {
	return MonthStart(t).AddDate(0, 1, 0)
}

func CurrentYearMonth() (int, int) {
	now := time.Now().In(Location())
	return now.Year(), int(now.Month())
}

func YesterdayBizDay(ts int64) int {
	return BizDay(StartOfBizDay(ts).AddDate(0, 0, -1).UnixMilli())
}

func NowMillis() int64 {
	return time.Now().UnixMilli()
}

func ToTime(ts int64) time.Time {
	return time.UnixMilli(ts).In(Location())
}

func BizDay(ts int64) int {
	t := ToTime(ts)
	return t.Year()*10000 + int(t.Month())*100 + t.Day()
}

func BizDayString(ts int64) string {
	return fmt.Sprintf("%08d", BizDay(ts))
}

func FormYearMonth(year int, month time.Month) time.Time {
	return time.Date(year, month, 1, 0, 0, 0, 0, Location())
}

func DayOfMonthFromBizDay(bizDay int) int {
	return bizDay % 100
}

func MonthRangeMillis(year int, month time.Month) (int64, int64) {
	start := FormYearMonth(year, month)
	next := start.AddDate(0, 1, 0)
	return start.UnixMilli(), next.UnixMilli()
}
