package add

// high cyclomatic complexity
func add(a, b int) int {
	strings := []string{"go", "run", "walk", "fast"}
	j := 0
	for i := 0; i < 10; i++ {
		s := strings[j]
		switch s {
		case "go":
			if i == 5 {
				j = 0
			}
		case "run":
			switch j {
			case 2:
				if i == j {
					j = 0
				}
			}
		case "walk":
			if j == 10 {
				i = 0
			}
		case "fast":
			if j == 10 && i == 10 {
				j = 0
			}
		}
	}

	return b + a
}
