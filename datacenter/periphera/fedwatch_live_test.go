package periphera

import (
	"testing"
	"time"
)

func TestParseFedWatchCMEPayload(t *testing.T) {
	body := []byte(`{
		"payload": [{
			"calculationTimestamp": "2026-07-01T08:30:00.000",
			"currentReportingRt": "350-375",
			"meetingDt": "2026-07-29",
			"rateRange": [
				{"lowerRt": 350, "upperRt": 375, "probability": 0.58},
				{"lowerRt": 325, "upperRt": 350, "probability": 0.38},
				{"lowerRt": 300, "upperRt": 325, "probability": 0.04}
			]
		}]
	}`)
	fw, err := parseFedWatch(body, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseFedWatch: %v", err)
	}
	if fw.Meeting != "7月 FOMC" || fw.MeetingDate != "2026-07-29" {
		t.Fatalf("unexpected meeting: %#v", fw)
	}
	assertOutcome(t, fw.Outcomes, 0, "维持不变", 58)
	assertOutcome(t, fw.Outcomes, 1, "降息25bp", 38)
	assertOutcome(t, fw.Outcomes, 2, "降息50bp", 4)
}

func TestParseFedWatchChoosesNearestFutureMeeting(t *testing.T) {
	body := []byte(`{
		"payload": [
			{
				"calculationTimestamp": "2026-07-01T08:30:00.000",
				"currentReportingRt": "350-375",
				"meetingDt": "2026-06-17",
				"rateRange": [{"lowerRt": 350, "upperRt": 375, "probability": 1}]
			},
			{
				"calculationTimestamp": "2026-07-01T08:30:00.000",
				"currentReportingRt": "350-375",
				"meetingDt": "2026-07-29",
				"rateRange": [{"lowerRt": 325, "upperRt": 350, "probability": 0.7}]
			}
		]
	}`)
	fw, err := parseFedWatch(body, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseFedWatch: %v", err)
	}
	if fw.MeetingDate != "2026-07-29" {
		t.Fatalf("expected nearest future meeting, got %s", fw.MeetingDate)
	}
	assertOutcome(t, fw.Outcomes, 0, "降息25bp", 70)
}

func TestParseFedWatchCompatPayload(t *testing.T) {
	body := []byte(`{
		"meeting": "9月 FOMC",
		"meeting_date": "2026-09-16",
		"outcomes": [
			{"label": "降息25bp", "prob": 62.5},
			{"label": "维持不变", "prob": 37.5}
		],
		"updated_at": 1780000000000
	}`)
	fw, err := parseFedWatch(body, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseFedWatch: %v", err)
	}
	if fw.Meeting != "9月 FOMC" || fw.UpdatedAt != 1780000000000 {
		t.Fatalf("unexpected compat parse: %#v", fw)
	}
	assertOutcome(t, fw.Outcomes, 0, "降息25bp", 62.5)
}

func TestParseInvestingFedWatchFirstCard(t *testing.T) {
	body := []byte(`
		<div class="cardWrapper">
			<div class="fedRateDate" id="cardName_0">Jul 29, 2026</div>
			<table class="genTbl openTbl fedRateTbl">
				<tbody>
					<tr>
						<td class="left">3.50 - 3.75 <span class="chartIcon"></span></td>
						<td>81.2%</td>
						<td>69.0%</td>
						<td>71.2%</td>
					</tr>
					<tr>
						<td class="left">3.75 - 4.00 <span class="chartIcon"></span></td>
						<td>18.8%</td>
						<td>31.0%</td>
						<td>28.8%</td>
					</tr>
				</tbody>
			</table>
			<div class="fedUpdate">Updated: Jul 02, 2026 10:05AM EDT </div>
		</div>
		<div class="cardWrapper">
			<div class="fedRateDate" id="cardName_1">Sep 16, 2026</div>
		</div>
	`)
	fw, err := parseInvestingFedWatch(body, time.Date(2026, 7, 2, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseInvestingFedWatch: %v", err)
	}
	if fw.Meeting != "7月 FOMC" || fw.MeetingDate != "2026-07-29" {
		t.Fatalf("unexpected meeting: %#v", fw)
	}
	assertOutcome(t, fw.Outcomes, 0, "降息25bp", 81.2)
	assertOutcome(t, fw.Outcomes, 1, "维持不变", 18.8)
	if fw.UpdatedAt == 0 {
		t.Fatal("expected updated_at to be parsed")
	}
}

func assertOutcome(t *testing.T, got []FedOutcome, i int, label string, prob float64) {
	t.Helper()
	if len(got) <= i {
		t.Fatalf("missing outcome %d in %#v", i, got)
	}
	if got[i].Label != label || got[i].Prob != prob {
		t.Fatalf("outcome %d = %#v, want %s %.2f", i, got[i], label, prob)
	}
}
