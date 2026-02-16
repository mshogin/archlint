package sample

// Repository интерфейс хранилища.
type Repository interface {
	Save(id string) error
	Get(id string) (string, error)
}

// Service бизнес-сервис.
type Service struct {
	repo Repository
}

// NewService создает новый сервис.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Process обрабатывает запрос.
func (s *Service) Process(id string) error {
	data, err := s.repo.Get(id)
	if err != nil {
		return err
	}

	result := transform(data)
	go s.notify(id, result)

	return s.repo.Save(id)
}

// notify отправляет уведомление (async).
func (s *Service) notify(id, result string) {
	_ = id
	_ = result
}

func transform(data string) string {
	return validate(data)
}

func validate(data string) string {
	return data
}

// ProcessWithDefer обрабатывает с defer.
func (s *Service) ProcessWithDefer(id string) error {
	defer s.cleanup(id)

	return s.repo.Save(id)
}

func (s *Service) cleanup(id string) {
	_ = id
}
