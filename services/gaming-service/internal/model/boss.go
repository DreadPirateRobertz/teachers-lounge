package model

import "time"

// BossSession is the live state of an active boss battle, persisted in Redis.
type BossSession struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	BossID       string     `json:"boss_id"`
	BossName     string     `json:"boss_name"`
	Topic        string     `json:"topic"`
	StudentHP    int        `json:"student_hp"`
	BossHP       int        `json:"boss_hp"`
	MaxBossHP    int        `json:"max_boss_hp"`
	Round        int        `json:"round"`        // 1-based round number
	MaxRounds    int        `json:"max_rounds"`
	ComboStreak  int        `json:"combo_streak"` // consecutive correct answers
	QuestionIDs  []string   `json:"question_ids"`
	CurrentIndex int        `json:"current_index"`
	TotalXP      int        `json:"total_xp"` // XP accumulated during battle
	Status       string     `json:"status"`   // "active", "victory", "defeat"
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// BossStartRequest is the body for POST /gaming/boss/start.
type BossStartRequest struct {
	UserID   string  `json:"user_id"`
	BossID   string  `json:"boss_id"`
	CourseID *string `json:"course_id,omitempty"`
}

// BossStartResponse is the response for POST /gaming/boss/start.
type BossStartResponse struct {
	Session  *BossSession `json:"session"`
	Question *Question    `json:"question"`
}

// BossSessionResponse is the response for GET /gaming/boss/sessions/{id}.
type BossSessionResponse struct {
	Session  *BossSession `json:"session"`
	Question *Question    `json:"question,omitempty"`
}

// BossAnswerRequest is the body for POST /gaming/boss/sessions/{id}/answer.
type BossAnswerRequest struct {
	UserID     string `json:"user_id"`
	QuestionID string `json:"question_id"`
	ChosenKey  string `json:"chosen_key"`
}

// BossAnswerResponse is the response for POST /gaming/boss/sessions/{id}/answer.
type BossAnswerResponse struct {
	Correct         bool     `json:"correct"`
	CorrectKey      string   `json:"correct_key"`
	Explanation     string   `json:"explanation"`
	DamageToBoss    int      `json:"damage_to_boss"`    // HP removed from boss (0 on wrong answer)
	DamageToStudent int      `json:"damage_to_student"` // HP removed from student (0 on correct)
	ComboStreak     int      `json:"combo_streak"`
	ComboMultiplier float64  `json:"combo_multiplier"`
	NewStudentHP    int      `json:"new_student_hp"`
	NewBossHP       int      `json:"new_boss_hp"`
	XPEarned        int      `json:"xp_earned"`           // XP for this answer
	TotalXP         int      `json:"total_xp"`            // cumulative XP for the battle
	BattleOver      bool     `json:"battle_over"`
	Victory         bool     `json:"victory"`
	VictoryXP       int      `json:"victory_xp,omitempty"`      // bonus XP awarded on victory
	NewXP           int64    `json:"new_xp,omitempty"`          // updated profile XP (on battle end)
	NewLevel        int      `json:"new_level,omitempty"`       // updated level (on battle end)
	LevelUp         bool     `json:"level_up,omitempty"`        // whether level increased (on battle end)
	BossesDefeated  int      `json:"bosses_defeated,omitempty"` // total bosses defeated (on victory)
	Taunt           string   `json:"taunt,omitempty"`           // boss taunt on wrong answer
	NextQuestion    *Question `json:"next_question,omitempty"`
	Session         *BossSession `json:"session"`
}
