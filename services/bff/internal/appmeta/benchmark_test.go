package appmeta_test

import (
	"strconv"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appmeta"
)

// Measurements are evidence about these synthetic shapes, not product quotas or
// a latency SLA. The 50k correctness assertion is independently covered above.
func BenchmarkValidateStructure(b *testing.B) {
	for _, shape := range []string{"chain", "star"} {
		for _, size := range []int{1000, 10000, 50000} {
			b.Run(shape+"/"+strconv.Itoa(size), func(b *testing.B) {
				s := appmeta.Structure{Application: appmeta.Application{ID: "app"}, Groups: make([]appmeta.Group, size)}
				for i := range s.Groups {
					var parent appmeta.ID
					if i > 0 {
						parent = s.Groups[0].ID
						if shape == "chain" {
							parent = s.Groups[i-1].ID
						}
					}
					s.Groups[i] = appmeta.Group{ID: appmeta.ID(strconv.Itoa(i)), ApplicationID: "app", ParentID: parent}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := appmeta.ValidateStructure(s); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
