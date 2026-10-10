package intent

import "testing"

func TestRecognizeEightAMOnly(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
	}{
		{"Bring an umbrella tomorrow 8am", UmbrellaReminder},
		{"bring an umbrella tomorrow 8:00", UmbrellaReminder},
		{"明天早上八点提醒带伞", UmbrellaReminder},
		{"明早八点带伞", UmbrellaReminder},
		{"bring an umbrella tomorrow at eight pm", Unknown},
		{"明天十八点提醒带伞", Unknown},
		{"明天晚上八点提醒带伞", Unknown},
		{"remind me to bring an umbrella today at 5pm", Unknown},
		{"bring an umbrella tomorrow at 18:00", Unknown},
		{"eighteen umbrellas tomorrow eight", Unknown},
		{"明天带伞", Unknown},
		{"open calculator", Unknown},
	}
	for _, tc := range cases {
		got := Recognize(tc.in)
		if got.Kind != tc.kind {
			t.Fatalf("%q: got %v want %v", tc.in, got.Kind, tc.kind)
		}
	}
}
