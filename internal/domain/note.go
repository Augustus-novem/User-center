package domain

const (
	NoteStatusPublished = "published"
	NoteStatusDeleted   = "deleted"
)

type NoteImage struct {
	URL       string
	SortOrder int
}

type Note struct {
	ID        int64
	AuthorID  int64
	Title     string
	Content   string
	Status    string
	Images    []NoteImage
	CreatedAt int64
	UpdatedAt int64
}

type NotePage struct {
	Items      []Note
	NextCursor string
	HasMore    bool
}
