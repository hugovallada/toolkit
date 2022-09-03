package toolkit

import "crypto/rand"

const (
	randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVXYZ0123456789_+"
)

// Tools is the type used to instantiate this module. Any variable of this type will have access
// to all the methods with the reciever *Tools
type Tools struct{}

/**
 * Cria 2 variáveis s, que é um slice de runes com o tamanho passado e r que será um slice de runes baseado na string passada
 * Itera sobre o s, pegando apenas o index
 * p recebe um inteiro randomico
 * x recebe p como um uint64 e y recebe o tamanho de r como um uint64
 * s recebe no seu index a rune que está no slice r na posição resultante do calculo de módulos
 * retorna o slice de runes s como uma string
 */
 // RandomString returns a string of random characters of lenght n, using randomStringSource
 // as the source for the string
func (t *Tools) RandomString(size int) string {
	s, r := make([]rune, size), []rune(randomStringSource)

	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}
