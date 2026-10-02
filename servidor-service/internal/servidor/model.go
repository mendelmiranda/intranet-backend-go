package servidor

// Servidor é o cadastro completo exposto pela view dbo.devops_servidor.
// Campos ausentes no banco saem como null.
type Servidor struct {
	Nome             *string  `json:"nome"`
	Matricula        *int     `json:"matricula"`
	CGM              int      `json:"cgm"`
	DtAdmissao       *string  `json:"dtAdmissao"`
	CodCargo         *int     `json:"codCargo"`
	Cargo            *string  `json:"cargo"`
	CodFuncao        *string  `json:"codFuncao"`
	Funcao           *string  `json:"funcao"`
	Servidor         *string  `json:"servidor"`
	CodLotacao       *int     `json:"codLotacao"`
	Lotacao          *string  `json:"lotacao"`
	Classe           *string  `json:"classe"`
	Regime           *string  `json:"regime"`
	CodPrevidencia   int      `json:"codPrevidencia"`
	Previdencia      string   `json:"previdencia"`
	PIS              *string  `json:"pis"`
	NIS              *string  `json:"nis"`
	CPF              *string  `json:"cpf"`
	RG               *string  `json:"rg"`
	DtExpRG          *string  `json:"dtExpRg"`
	OrgaoExpRG       *string  `json:"orgaoExpRg"`
	DtNascimento     *string  `json:"dtNascimento"`
	Sexo             *string  `json:"sexo"`
	Natural          *string  `json:"natural"`
	Nacionalidade    *string  `json:"nacionalidade"`
	EstCivil         *string  `json:"estCivil"`
	Endereco         *string  `json:"endereco"`
	Numero           *int     `json:"numero"`
	Logradouro       *string  `json:"logradouro"`
	Bairro           *string  `json:"bairro"`
	Municipio        *string  `json:"municipio"`
	UF               *string  `json:"uf"`
	CEP              *int     `json:"cep"`
	Celular          *string  `json:"celular"`
	CaixaPostal      string   `json:"caixaPostal"`
	Pai              *string  `json:"pai"`
	Mae              *string  `json:"mae"`
	HoraMensal       *int     `json:"horaMensal"`
	HoraSemanal      *int     `json:"horaSemanal"`
	DtUltAlteracao   string   `json:"dtUltAlteracao"`
	HoraUltAlteracao string   `json:"horaUltAlteracao"`
	DtCadastro       *string  `json:"dtCadastro"`
	Ano              *int     `json:"ano"`
	Mes              *int     `json:"mes"`
	SalarioBase      *float64 `json:"salarioBase"`
	Agencia          *int     `json:"agencia"`
	DvAgencia        *string  `json:"dvAgencia"`
	Conta            *string  `json:"conta"`
	DvConta          *string  `json:"dvConta"`
	Cod              *int     `json:"cod"`
	Banco            *string  `json:"banco"`
	Email            *string  `json:"email"`
	Login            *string  `json:"login"`
	Quadro           *string  `json:"quadro"`
	VinculoRais      *string  `json:"vinculoRais"`
	Ativo            string   `json:"ativo"`
	MatriculaSistema *int     `json:"matriculaSistema"`
}

// Resultado agrupa os cadastros que atenderam ao filtro.
type Resultado struct {
	Total      int        `json:"total"`
	Servidores []Servidor `json:"servidores"`
}

// Filtro combina os critérios informados. Campos vazios não entram na consulta.
// CPF já vem só com dígitos. Nome é o texto livre, sem curingas.
type Filtro struct {
	CPF       string
	Nome      string
	Matricula *int
}

func (f Filtro) vazio() bool {
	return f.CPF == "" && f.Nome == "" && f.Matricula == nil
}
