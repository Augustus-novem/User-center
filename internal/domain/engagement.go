package domain

const CommentStatusPublished = "published"

type NoteLike struct {
	ID        int64
	UserID    int64
	NoteID    int64
	CreatedAt int64
}

type Comment struct {
	ID        int64
	NoteID    int64
	UserID    int64
	Content   string
	Status    string
	CreatedAt int64
}

type CommentPage struct {
	Items      []Comment
	NextCursor string
	HasMore    bool
}
