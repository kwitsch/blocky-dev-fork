package resolver

import (
	"github.com/0xERR0R/blocky/config"
	"github.com/0xERR0R/blocky/model"
	. "github.com/0xERR0R/blocky/model"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("EdeResolver", func() {
	var (
		sut        *EdeResolver
		sutConfig  config.Config
		m          *MockResolver
		mockAnswer *dns.Msg
	)

	BeforeEach(func() {
		mockAnswer = new(dns.Msg)
	})

	JustBeforeEach(func() {
		m = &MockResolver{}
		m.On("Resolve", mock.Anything).Return(&Response{Res: mockAnswer, RType: model.ResponseTypeCUSTOMDNS}, nil)

		sut = NewEdeResolver(sutConfig, m).(*EdeResolver)

	})

	When("Ede is disabled", func() {
		BeforeEach(func() {
			sutConfig = config.Config{
				EdeEnabled: false,
			}
		})
		It("Should delegate to next resolver if request query has other type", func() {
			resp, err := sut.Resolve(newRequest("example.com", dns.Type(dns.TypeA)))
			Expect(err).Should(Succeed())
			Expect(resp.Res.Rcode).Should(Equal(dns.RcodeSuccess))
			Expect(resp.RType).Should(Equal(ResponseTypeRESOLVED))
			Expect(resp.Res.Answer).Should(BeEmpty())
			Expect(resp.Res.Extra).Should(BeEmpty())

			// delegated to next resolver
			Expect(m.Calls).Should(HaveLen(1))
		})
		It("Configure should output deactivated", func() {
			c := sut.Configuration()
			Expect(c).Should(HaveLen(1))
			Expect(c[0]).Should(Equal("deactivated"))
		})
	})
	When("Ede is enabled", func() {
		BeforeEach(func() {
			sutConfig = config.Config{
				EdeEnabled: true,
			}
		})
		It("Should delegate to next resolver if request query has other type", func() {
			resp, err := sut.Resolve(newRequest("example.com", dns.Type(dns.TypeA)))
			Expect(err).Should(Succeed())
			Expect(resp.Res.Rcode).Should(Equal(dns.RcodeSuccess))
			Expect(resp.RType).Should(Equal(ResponseTypeRESOLVED))
			Expect(resp.Res.Answer).Should(BeEmpty())
			Expect(resp.Res.Extra).Should(HaveLen(1))
			opt := getEDE(resp.Res.Extra)
			Expect(opt).ShouldNot(BeNil())
			ede, ok := opt.Option[0].(*dns.EDNS0_EDE)
			Expect(ok).Should(BeTrue())
			Expect(ede.InfoCode).Should(Equal(dns.ExtendedErrorCodeForgedAnswer))

		})
		It("Configure should output activated", func() {
			c := sut.Configuration()
			Expect(c).Should(HaveLen(1))
			Expect(c[0]).Should(Equal("activated"))
		})
	})
})

func getEDE(rrs []dns.RR) *dns.OPT {
	for _, extra := range rrs {

		switch extra.(type) {
		case *dns.OPT:
			if res, ok := extra.(*dns.OPT); ok {
				return res
			}
		}
	}
	return nil
}
