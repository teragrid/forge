import sys
raw = open(r'internal\cli\cmdship\ship.go', 'rb').read()
old = (b'type Checkpoint struct {\r\n'
       b'\tName     string        `json:"name"`\r\n'
       b'\tStatus   string        `json:"status"` // "ok", "skipped", "warning", "fail"\r\n'
       b'\tDetail   string        `json:"detail"`\r\n'
       b'\tApproved *bool         `json:"approved,omitempty"` // nil=yolo/not-gated; true=approved; false=rejected\r\n'
       b'\tDebate   *DebateResult `json:"debate,omitempty"`   // populated when --yolo self-debate runs\r\n'
       b'}')
new = (b'type Checkpoint struct {\r\n'
       b'\tName        string        `json:"name"`\r\n'
       b'\tStatus      string        `json:"status"` // "ok", "skipped", "warning", "fail"\r\n'
       b'\tDetail      string        `json:"detail"`\r\n'
       b'\tAutoAdvance bool          `json:"auto_advance,omitempty"` // G-009: Code sets true when all tasks done\r\n'
       b'\tApproved    *bool         `json:"approved,omitempty"` // nil=yolo/not-gated; true=approved; false=rejected\r\n'
       b'\tDebate      *DebateResult `json:"debate,omitempty"`   // populated when --yolo self-debate runs\r\n'
       b'}')
if old not in raw:
    print("NOT FOUND"); sys.exit(1)
open(r'internal\cli\cmdship\ship.go', 'wb').write(raw.replace(old, new, 1))
print("AutoAdvance field added")
