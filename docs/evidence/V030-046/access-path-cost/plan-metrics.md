D=definitions relation; I=instances relation; X=index-only scan node without Relation Name. S/L=Shared Hit Blocks/Local Hit Blocks. Sort used follows raw EXPLAIN units; — means field/node absent. Values copied from raw EXPLAIN, no ranking or SLA claim.

| variant/query/sample | Execution ms | Planning ms | Top rows | Scan rows × loops | Top hits S/L | Scan hits S/L | Sort used/type | index_bytes |
| --- | ---: | ---: | ---: | --- | --- | --- | --- | ---: |
| existing/first/0 | 13.734 | 0.172 | 20.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 45/Memory | 0 |
| existing/first/1 | 15.323 | 0.384 | 20.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 45/Memory | 0 |
| existing/first/2 | 14.883 | 0.3 | 20.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 45/Memory | 0 |
| existing/deep/0 | 16.27 | 0.167 | 20.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 532/Memory | 0 |
| existing/deep/1 | 15.387 | 0.181 | 20.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 532/Memory | 0 |
| existing/deep/2 | 17.216 | 0.186 | 20.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 532/Memory | 0 |
| existing/fingerprint/0 | 16.774 | 0.215 | 1000.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 407/Memory | 0 |
| existing/fingerprint/1 | 18.061 | 0.223 | 1000.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 407/Memory | 0 |
| existing/fingerprint/2 | 18.076 | 0.278 | 1000.0 | D:Seq=1.0×1; I:Seq=1000.0×1 | 0/1 | D:Seq=0/1; I:Seq=0/0 | 407/Memory | 0 |
| prefix/first/0 | 3.654 | 0.125 | 20.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 45/Memory | 761856 |
| prefix/first/1 | 3.752 | 0.107 | 20.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 45/Memory | 761856 |
| prefix/first/2 | 3.645 | 0.092 | 20.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 45/Memory | 761856 |
| prefix/deep/0 | 3.793 | 0.091 | 20.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 532/Memory | 761856 |
| prefix/deep/1 | 4.018 | 0.091 | 20.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 532/Memory | 761856 |
| prefix/deep/2 | 3.94 | 0.123 | 20.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 532/Memory | 761856 |
| prefix/fingerprint/0 | 4.319 | 0.084 | 1000.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 407/Memory | 761856 |
| prefix/fingerprint/1 | 4.143 | 0.083 | 1000.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 407/Memory | 761856 |
| prefix/fingerprint/2 | 4.948 | 0.127 | 1000.0 | D:Seq=1.0×1; I:Index=1000.0×1 | 0/1004 | D:Seq=0/1; I:Index=0/1003 | 407/Memory | 761856 |
| ordered/first/0 | 0.15 | 0.198 | 20.0 | I:Index=20.0×1; D:Seq=1.0×1 | 0/24 | I:Index=0/23; D:Seq=0/1 | — | 9551872 |
| ordered/first/1 | 0.12 | 0.087 | 20.0 | I:Index=20.0×1; D:Seq=1.0×1 | 0/24 | I:Index=0/23; D:Seq=0/1 | — | 9551872 |
| ordered/first/2 | 0.115 | 0.083 | 20.0 | I:Index=20.0×1; D:Seq=1.0×1 | 0/24 | I:Index=0/23; D:Seq=0/1 | — | 9551872 |
| ordered/deep/0 | 4.217 | 0.084 | 20.0 | I:Index=1000.0×1; D:Seq=1.0×1 | 0/903 | I:Index=0/902; D:Seq=0/1 | — | 9551872 |
| ordered/deep/1 | 3.573 | 0.093 | 20.0 | I:Index=1000.0×1; D:Seq=1.0×1 | 0/1015 | I:Index=0/1014; D:Seq=0/1 | — | 9551872 |
| ordered/deep/2 | 3.693 | 0.083 | 20.0 | I:Index=1000.0×1; D:Seq=1.0×1 | 0/1015 | I:Index=0/1014; D:Seq=0/1 | — | 9551872 |
| ordered/fingerprint/0 | 4.16 | 0.137 | 1000.0 | I:Index=1000.0×1; D:Seq=1.0×1 | 0/1015 | I:Index=0/1014; D:Seq=0/1 | — | 9551872 |
| ordered/fingerprint/1 | 4.116 | 0.171 | 1000.0 | I:Index=1000.0×1; D:Seq=1.0×1 | 0/1015 | I:Index=0/1014; D:Seq=0/1 | — | 9551872 |
| ordered/fingerprint/2 | 4.948 | 0.097 | 1000.0 | I:Index=1000.0×1; D:Seq=1.0×1 | 0/1015 | I:Index=0/1014; D:Seq=0/1 | — | 9551872 |
