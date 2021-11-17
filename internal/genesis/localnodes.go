package genesis

// LocalHarmonyAccounts are the accounts for the initial genesis nodes used for local test.
// localnetV0 最开始都使用这组地址 准备64个节点
var LocalHarmonyAccounts = []DeployAccount{
	{Index: " 0 ", Address: "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy", BLSPublicKey: "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204"},
	{Index: " 1 ", Address: "one1m6m0ll3q7ljdqgmth2t5j7dfe6stykucpj2nr5", BLSPublicKey: "40379eed79ed82bebfb4310894fd33b6a3f8413a78dc4d43b98d0adc9ef69f3285df05eaab9f2ce5f7227f8cb920e809"},
	{Index: " 2 ", Address: "one12fuf7x9rgtdgqg7vgq0962c556m3p7afsxgvll", BLSPublicKey: "02c8ff0b88f313717bc3a627d2f8bb172ba3ad3bb9ba3ecb8eed4b7c878653d3d4faf769876c528b73f343967f74a917"},
	{Index: " 3 ", Address: "one16qsd5ant9v94jrs89mruzx62h7ekcfxmduh2rx", BLSPublicKey: "ee2474f93cba9241562efc7475ac2721ab0899edf8f7f115a656c0c1f9ef8203add678064878d174bb478fa2e6630502"},
	{Index: " 4 ", Address: "one1pf75h0t4am90z8uv3y0dgunfqp4lj8wr3t5rsp", BLSPublicKey: "e751ec995defe4931273aaebcb2cd14bf37e629c554a57d3f334c37881a34a6188a93e76113c55ef3481da23b7d7ab09"},
	{Index: " 5 ", Address: "one1est2gxcvavmtnzc7mhd73gzadm3xxcv5zczdtw", BLSPublicKey: "776f3b8704f4e1092a302a60e84f81e476c212d6f458092b696df420ea19ff84a6179e8e23d090b9297dc041600bc100"},
	{Index: " 6 ", Address: "one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj", BLSPublicKey: "2d61379e44a772e5757e27ee2b3874254f56073e6bd226eb8b160371cc3c18b8c4977bd3dcb71fd57dc62bf0e143fd08"},
	{Index: " 7 ", Address: "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9", BLSPublicKey: "c4e4708b6cf2a2ceeb59981677e9821eebafc5cf483fb5364a28fa604cc0ce69beeed40f3f03815c9e196fdaec5f1097"},
	{Index: " 8 ", Address: "one1d2rngmem4x2c6zxsjjz29dlah0jzkr0k2n88wc", BLSPublicKey: "86dc2fdc2ceec18f6923b99fd86a68405c132e1005cf1df72dca75db0adfaeb53d201d66af37916d61f079f34f21fb96"},
	{Index: " 9 ", Address: "one1658znfwf40epvy7e46cqrmzyy54h4n0qa73nep", BLSPublicKey: "49d15743b36334399f9985feb0753430a2b287b2d68b84495bbb15381854cbf01bca9d1d9f4c9c8f18509b2bfa6bd40f"},

	{Index: " 10 ", Address: "one1z05g55zamqzfw9qs432n33gycdmyvs38xjemyl", BLSPublicKey: "95117937cd8c09acd2dfae847d74041a67834ea88662a7cbed1e170350bc329e53db151e5a0ef3e712e35287ae954818"},
	{Index: " 11 ", Address: "one1ljznytjyn269azvszjlcqvpcj6hjm822yrcp2e", BLSPublicKey: "68ae289d73332872ec8d04ac256ca0f5453c88ad392730c5741b6055bc3ec3d086ab03637713a29f459177aaa8340615"},
	{Index: " 12 ", Address: "one1axr7r734fjgtfkpk536hkvehvsajesh5ak2p9s", BLSPublicKey: "62007fba92486248f9df934a8ceb5656e63118b03a881950570420f6db8a51b136445796e5c847963d8f52136637f990"},
	{Index: " 13 ", Address: "one1320aecgmryfahgchx575jktwgx22q38ees3wc4", BLSPublicKey: "024910e70c88c7aa3fca7fdb7b9cded9dfb466d4fd0f4ec03b70d4ed39333660f57ea6bcca7d0e1570a846a735764d00"},
	{Index: " 14 ", Address: "one14eewn7t704lmxds76mtmqjqz6nvrc26qaxmllc", BLSPublicKey: "0620ee33548e6acdf93ad83d01ddd138fab24cee4fe5cf8caacd9e1b853d3987303b639abdb41916d8a0aea0599eb290"},
	{Index: " 15 ", Address: "one14wg68nspdarq8nrf96uaszkfd8mjpg3nap763a", BLSPublicKey: "1b2c4eef6bf1601f382f9d25f695cd53633473b9617a9ea107d219c0623a9cf88898b21523b37750ece1d50a7c39e18a"},
	{Index: " 16 ", Address: "one15qa2fjxm4tzmlk7ymw2z9mxgjwa4h4f7fmd2m7", BLSPublicKey: "1d5cd1e5e0da472a165d6bfcea9d8894d2a248dd9bd7510f614033b4a0f618161b25a868355b038a0b994af1cf9a0e10"},
	{Index: " 17 ", Address: "one1fn56tly05jvtuyze4805mmrl5cjlerdcvlaqld", BLSPublicKey: "3203908ade069506b06ff71881f8c147b72d034945a33ed14460890a72652bf7248c978039c1634e0bacb8469e7bf118"},
	{Index: " 18 ", Address: "one1l6aka3yz0wkxqzh2x5g36x25daqjwrq7066y8t", BLSPublicKey: "478eacd9e8568fc45f78e42e53b6c1dc0db2ce24e3728de686313e2803ec0c0fb5de9ebace5b913f6777370c8c53c418"},
	{Index: " 19 ", Address: "one1pl728xtegxwa2vtczxe4lzgphftufqughk6f82", BLSPublicKey: "90db0a71f72608ea2744b8128660c55735c3204c32e95636feef40884e2d1ad12912f364070e1cca778ff863de424b10"},
	{Index: " 20 ", Address: "one1s63qmh9g2538e02lxy2k42q7qkvr2mcn00eh5n", BLSPublicKey: "b1425193464c7f6cd0d38bdd2b54899225b9cbad1b62cca6aa7c2beb386f5398353b4fc42f7329934473f6174bff6688"},
	{Index: " 21 ", Address: "one1t0ar9spssq3hnhagc8xe7tz3lcrm8cl4qrncte", BLSPublicKey: "b293a0a44ebf3f496cfca8767cf2b70dd67f00a4c36ca1bbd3a0958c20f9e2e224b7babf1b05df46c0d1af1a1119f090"},
	{Index: " 22 ", Address: "one1td8h4udjm77lpcpvav6d89s5566djrxnapsarc", BLSPublicKey: "e410f7006022b6b0ae52c41527ff4c916eac9743abd26f96dabf1a952233d6ac29062ddd42e16b6fc8ac09a540f5aa18"},
	{Index: " 23 ", Address: "one1wkudmcznma2s4sjcmerwg9zx9fjqh88n53sgtq", BLSPublicKey: "e653fd4ddf8d51baeaa6dbd221a31338c320f1a4f469dc53a4728c9c5e2eec504399ca88bc8c8529f6b2dfcdd0ace580"},
	{Index: " 24 ", Address: "one1x6fs6ftkg6vykqle9mhxkc9clat5ewpdeng7ry", BLSPublicKey: "f41d0a625fe219f29c3b68f9c6fc7d6bc60089f7c972d2d1c4adba1d49ad55ad1559776a8f08cc0be3df0f14f79b2308"},
	// 最新节点
	{Index: " 25 ", Address: "one103q7qe5t2505lypvltkqtddaef5tzfxwsse4z7", BLSPublicKey: "16513c487a6bb76f37219f3c2927a4f281f9dd3fd6ed2e3a64e500de6545cf391dd973cc228d24f9bd01efe94912e714"},
	{Index: " 26 ", Address: "one129r9pj3sk0re76f7zs3qz92rggmdgjhtwge62k", BLSPublicKey: "1c1fb28d2de96e82c3d9b4917eb54412517e2763112a3164862a6ed627ac62e87ce274bb4ea36e6a61fb66a15c263a06"},
	{Index: " 27 ", Address: "one1d7jfnr6yraxnrycgaemyktkmhmajhp8kl0yahv", BLSPublicKey: "2d3d4347c5a7398fbfa74e01514f9d0fcd98606ea7d245d9c7cfc011d472c2edc36c738b78bd115dda9d25533ddfe301"},
	{Index: " 28 ", Address: "one1ghkz3frhske7emk79p7v2afmj4a5t0kmjyt4s5", BLSPublicKey: "4235d4ae2219093632c61db4f71ff0c32bdb56463845f8477c2086af1fe643194d3709575707148cad4f835f2fc4ea05"},
	{Index: " 29 ", Address: "one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg", BLSPublicKey: "52ecce5f64db21cbe374c9268188f5d2cdd5bec1a3112276a350349860e35fb81f8cfe447a311e0550d961cf25cb988d"},
	{Index: " 30 ", Address: "one1p7ht2d4kl8ve7a8jxw746yfnx4wnfxtp8jqxwe", BLSPublicKey: "576d3c48294e00d6be4a22b07b66a870ddee03052fe48a5abbd180222e5d5a1f8946a78d55b025de21635fd743bbad90"},
	{Index: " 31 ", Address: "one1a50tun737ulcvwy0yvve0pvu5skq0kjargvhwe", BLSPublicKey: "63f479f249c59f0486fda8caa2ffb247209489dae009dfde6144ff38c370230963d360dffd318cfb26c213320e89a512"},

	{Index: " 32 ", Address: "one1r4zyyjqrulf935a479sgqlpa78kz7zlcg2jfen", BLSPublicKey: "678ec9670899bf6af85b877058bea4fc1301a5a3a376987e826e3ca150b80e3eaadffedad0fedfa111576fa76ded980c"},
	{Index: " 33 ", Address: "one10k2f9at7pux0skvr62mx4y42tuuyt822xygnzz", BLSPublicKey: "a547a9bf6fdde4f4934cde21473748861a3cc0fe8bbb5e57225a29f483b05b72531f002f8187675743d819c955a86100"},
	{Index: " 34 ", Address: "one10txx4r428lagq68cnyzkk2pxyst39z6pzlcs7h", BLSPublicKey: "b179c4fdc0bee7bd0b6698b792837dd13404d3f985b59d4a9b1cd0641a76651e271518b61abbb6fbebd4acf963358604"},
	{Index: " 35 ", Address: "one13e76ewz3tnw0sjlqyrglpt3szzfdqrffhng8qc", BLSPublicKey: "ca86e551ee42adaaa6477322d7db869d3e203c00d7b86c82ebee629ad79cb6d57b8f3db28336778ec2180e56a8e07296"},
	{Index: " 36 ", Address: "one14856ge33f7s3m409e9tx2gee3amr5z7qz67k4h", BLSPublicKey: "eca09c1808b729ca56f1b5a6a287c6e1c3ae09e29ccf7efa35453471fcab07d9f73cee249e2b91f5ee44eb9618be3904"},
	{Index: " 37 ", Address: "one15ul5cgh5xqx7rsccn98kwzu7m22xvhpzjszfwj", BLSPublicKey: "f47238daef97d60deedbde5302d05dea5de67608f11f406576e363661f7dcbc4a1385948549b31a6c70f6fde8a391486"},
	{Index: " 38 ", Address: "one15wzgg7rgtx0h3getn4d9cwx34yhwj67x58fwfz", BLSPublicKey: "fc4b9c535ee91f015efff3f32fbb9d32cdd9bfc8a837bb3eee89b8fff653c7af2050a4e147ebe5c7233dc2d5df06ee0a"},
	{Index: " 39 ", Address: "one16tg5dtuputgwl2g30wags84updp7x3ft266hn7", BLSPublicKey: "0190e26487fc6b9ad7dfdd8353972965ccc0ca246085b1fa9a5a2ce658ac9be4d18ec9c618b9bb112ff560323678b694"},
	{Index: " 40 ", Address: "one16utw0sp67gc7thx2sn7eg7g7nwdu2upl7t47p9", BLSPublicKey: "0506fac693389701455d6ef9cf3c4b2b6d7a790de743533a655ba57e5cc49862a7f0b02447ef6c7e8dd6b4af8336d78c"},
	{Index: " 41 ", Address: "one1c30hvyvveypatg76vuwng9zk2g403edr754cp4", BLSPublicKey: "081007d21f47f4543cdf68408cd96feed359318259e12a09217c364ce16f8042eae582fadd354125fdf2acc99d781d0c"},
	{Index: " 42 ", Address: "one1c6mcxz90h8zuv69zqsm3smydtj7mzmw2kuf0q3", BLSPublicKey: "11508d7a4c21225901da44831dd31aca3da81a8746dbab8465bc384be17e34eaac1114ff6eda15c4dbfba34b730cd884"},
	{Index: " 43 ", Address: "one1ec2y5c2632ght2gnegkxegxda56sqpy65n4upv", BLSPublicKey: "171034929bf6bb461349d3e834838a32063810ee32cf889815001c2a2fd9f51420e9298b2bb0113aacc61010ee7bcc80"},
	{Index: " 44 ", Address: "one1fqsk7t0s2erpsyxl8gn220q38puf8f73ckexe0", BLSPublicKey: "1e58f0d1f8a3afb0d7ef00f82c19e00646a9590e0f025a4e216d2db9b716967d879ba1193090510a6c37c088aa804c10"},
	{Index: " 45 ", Address: "one1h48vruc0njj6mdm7w2jgny7akv29yea0yus8p6", BLSPublicKey: "1ef34fbccc876a2515841dcf2e792c81da9abd33e77d9d852b3e589c859b28f04bcd219601c5b259b4cd36176fe9830c"},
	{Index: " 46 ", Address: "one1j2ddczwva844g7r2dnafl0dav0dd3xe4jwfctf", BLSPublicKey: "281dd64710ec2ff1f3437ebdc2dd9073ad6c6dec968915dfb19a78a90bbe9e479c2907bf401649352b1c7c66fd0ff80c"},
	{Index: " 47 ", Address: "one1jevta64ry2fzs2xfwq4t9vqupfjp5mku8hsq40", BLSPublicKey: "39d1d1745f0499d6b2df49c90851115799382b16fffa8270fada90e924a28485f59f332739a171a41756a6d15acd2290"},

	{Index: " 48 ", Address: "one1jyfmuc9kxqq5g0wk58uf8vzduwpqj4dpuswl65", BLSPublicKey: "488cbb798d7363df2386d5b50182b9fe9314a8617b8e5f262dcedddf0b9ae10446e03f122968fcca809864710045e008"},
	{Index: " 49 ", Address: "one1kpdmdqn56gjxtgcj0nfund9ekc0nfggwcwt8jw", BLSPublicKey: "49bf6f7536e9098ac4281ba65680a7141d0ba8aef4b8549e975fd9e219aa32b0bb098c07bc25e6e2cba2bc6693c3a594"},
	{Index: " 50 ", Address: "one1m243x0hg04lkndcw3vsr4stlfu0ulg4u8ss9lz", BLSPublicKey: "5a972254ba7700eedd77b5692ca898e9b9c13a5e0be77d728871636cc54ef16356d374f9fa7c8e7c96c0fe8d2abaf600"},
	{Index: " 51 ", Address: "one1mds369y2pvp9203uzz0ernd4gf8kpd5xyu87t0", BLSPublicKey: "6db08619244a9f15a3485325c310f75cf16055054729bc2438c7073c2bec42cbda30cbf5004811c975c62d2ab1c0fc84"},
	{Index: " 52 ", Address: "one1msvcdj05rgnqdtyr6aq57s2jnqntd8jacelnul", BLSPublicKey: "715d04f27841af26ee633e2cdee5c5ac903ad43207634bfa70010d1eac55bb68cdf67a71aa89942719054e8d7d3e9f14"},
	{Index: " 53 ", Address: "one1p8t6e8l37p9m8uteyu9wl2nlc5npwyu5hulrud", BLSPublicKey: "7ee2483290128a7723261dbfc0fe369eeae6e32c472d4494f99afbba5f331ce78deb490d2e21f4e5ec14f126781e7908"},
	{Index: " 54 ", Address: "one1pzddr2jsgegppe83r2e5nfjr5nnsz5yasrfz26", BLSPublicKey: "7f840f9f085e0a7a737ce2ce936c4eafb37694a5b15d6cbaf137a9996c7969dab6a45d8e6376709f916e2a2a23465204"},
	{Index: " 55 ", Address: "one1q0vk09mldphrlshhwg0sjwmgdhlsf9u5fjhrd3", BLSPublicKey: "8407b7b5b5c07047f593bb4b5db6d4b14f3d369c857a2ef17c32fbe412754c3a3ca7e563e13a25e516ad651a06cf0910"},

	{Index: " 56 ", Address: "one1qlzqg0q3y0k0s9y2frp858vcgh2zkvr23ykrkg", BLSPublicKey: "b625345d167e7079a5a259111fc2515bc177e270a2f17fde702a5892fd42a5f5bb9ff3adf629d7bc6245a5defaf76a8c"},
	{Index: " 57 ", Address: "one1swn80hdgzu9v9ly4jt0ndzw7z3u9978a2hd6mj", BLSPublicKey: "be87578340b25da6981dab7a963ed21633a223427cc41b5589523e9095161cc123d90157453fb5086b5547c6e1203e10"},
	{Index: " 58 ", Address: "one1ths4jte054tmsy6a0np6tk9p4skfnj5xgqtpqs", BLSPublicKey: "c83d2a17925696f34e0f9d602d3861914fb8d622ca904b77dc0f220e77ffb992a64c697ac3d740dbee2113dded632318"},
	{Index: " 59 ", Address: "one1tx2f83qcvrq7tck88xrdmjdhumzmfj8ra6ykt0", BLSPublicKey: "c87507ff1e76de3f655ed903d0aae4937bc2405b5c4665dafeb507e6bdd6afba75448b46231f0f19f4a89755de3a7690"},

	{Index: " 60 ", Address: "one1ukz9s0f9y7mn43zcdpvuxw24cx47225peqwh3q", BLSPublicKey: "e05ffa26b083fddb4facc03fe97b04b225a9e1c3d9324f2a48ae25fceb08971a6ab43105f497e4f2276ed2d7c2b33b88"},
	{Index: " 61 ", Address: "one1w4q3uf242sun9s8fj0z0pxqzttgvmxe5am34ec", BLSPublicKey: "e7aae104374d952d08377aa677aecd72fbcc27abefc371c04605d39337a578da69dc12c026fd08c30d858857f8489404"},
	{Index: " 62 ", Address: "one1z7c7qmpfaqz363fqcg00ll2xhewsy76aawy32j", BLSPublicKey: "e9db92511dff3fbad8d26340e94cd468159c8188c9f896f3502584f9a976a5b728467c5393a3b6659af52ea86277bf08"},
	{Index: " 63 ", Address: "one1znqjvhlx8nzjvw2t0nkzzs6mcaxdy43xs5mspq", BLSPublicKey: "ed83df908af0680c605faea302f320451b37c8a169dbe63ee9ae5fde223a3f98f9b6efa60f9bc0d184f08e2d02448904"},
}

// LocalFnAccounts are the accounts for the initial FN used for local test.
//var LocalFnAccounts = []DeployAccount{
//	{Index: " 0 ", Address: "one1a50tun737ulcvwy0yvve0pvu5skq0kjargvhwe", BLSPublicKey: "52ecce5f64db21cbe374c9268188f5d2cdd5bec1a3112276a350349860e35fb81f8cfe447a311e0550d961cf25cb988d"},
//	{Index: " 1 ", Address: "one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg", BLSPublicKey: "a547a9bf6fdde4f4934cde21473748861a3cc0fe8bbb5e57225a29f483b05b72531f002f8187675743d819c955a86100"},
//	{Index: " 2 ", Address: "one103q7qe5t2505lypvltkqtddaef5tzfxwsse4z7", BLSPublicKey: "678ec9670899bf6af85b877058bea4fc1301a5a3a376987e826e3ca150b80e3eaadffedad0fedfa111576fa76ded980c"},
//	{Index: " 3 ", Address: "one129r9pj3sk0re76f7zs3qz92rggmdgjhtwge62k", BLSPublicKey: "63f479f249c59f0486fda8caa2ffb247209489dae009dfde6144ff38c370230963d360dffd318cfb26c213320e89a512"},
//}
// fn 地址,bls要额外生成,
var LocalFnAccounts = []DeployAccount{
	{Index: " 0 ", Address: "one12gfgn40d2x2uawykuwknfkhklzstr4x8avs0k8", BLSPublicKey: "00d600ed10af6905fe2ff63b3112d824ee6607adb82063c5cdee31781a94d1ebbc2bead7b307980de424ce641d14d594"},
	{Index: " 1 ", Address: "one12lay9uzjhydmscyp3wh9m7quehw73n2x9s85qu", BLSPublicKey: "52e533007d9e08a2cce4301774a1b9f03e57c3e553baf782275efd4e3236dc785f3729b8d9404d61f4974766d3d35700"},
	{Index: " 2 ", Address: "one13k52gdtgz62ewphuk6ekva858qphe4ffcgcnhr", BLSPublicKey: "6fda854ab94d722a5dbb3b0ffa9430ddda22e7489050702f8fd95c60f52db21be12b84960e87001055d5e744156b688c"},
	{Index: " 3 ", Address: "one1cdcjpqmft4rvt70jas37cuhx4xmpjcqfjd334t", BLSPublicKey: "8f744d023abe53030865c9e973e9ec146b16d5294a7f68455dc1829a9ae8109025a0b1170717f5709e2b3739a9dfcb08"},
	{Index: " 4 ", Address: "one1jwsxyk0yamylpl9d8pt6sxhtdlvj0sh4l9w3th", BLSPublicKey: "9201986fb2b34ceba38841335974d5618f79f6fe147e352533e8419d34407f0c265f2136a81b756a7a66e7ace72fb294"},
	{Index: " 5 ", Address: "one1m2r0vd32xk47v03262xhm04jsn0fej4yscml8h", BLSPublicKey: "9707ee8c8008dcf0955f9525c9d43aefdbda6fbae0afa55940ae8bce22149933dac46dbbb2d81207daebfcf3bb7e6c88"},
	{Index: " 6 ", Address: "one1p7qhkh2g8lxamn8r4a7dy68fcqt0y4y6km7gzk", BLSPublicKey: "c33b121bd16f2cf502dab305380f02238907a44cacb1662366fb47058c8824cccec1486312bfe8d8ed7614acc2d5c018"},
	{Index: " 7 ", Address: "one1q43jr2cvvwf5ufhsxakj6sg5m04s5w2w2gcrtg", BLSPublicKey: "d3d32d50191d861980a4af5578d80a88bfb9d0c5f834d4e94285c632477a7b661339736a678c5172b149135b13693704"},
	{Index: " 8 ", Address: "one1uv0cetdyt6fh064nud6guyc8thesguc9pg7weq", BLSPublicKey: "d7fd133b1633786ac516e93fdf503a4e055d9c9177a190f8c090fbba39f160310b19f49423bf771672b7437b92de5800"},
	{Index: " 9 ", Address: "one1yswr3r7ftxx8xntmp02tmm2as0md0svpd5j0v8", BLSPublicKey: "f321dd31c8ab3aa2efe09e194f07a4fa9195aa8aef1104d36234172ef6d1d6d365cc043196f40d763654c822f264c88c"},
	{Index: " 10 ", Address: "one1zg0hdhcuz3vfu49v8z8mujpj79raruxunhpfnk", BLSPublicKey: "4f8df320d3aea7cb1a4a55a175c721cee67f2f5cac6d6f5d66bc77d973e635e1af8c891db3a2856d3415a03ef0502f84"},

	{Index: " 11 ", Address: "one1wfx08nu6pjlkw0dysla78jljuu6ygk4y7kekt0", BLSPublicKey: "19c78e0ab182af9f1e75108487993cc24d8fbad4fc9fa04a3d186ed654f520899fa5d5bfaee3ef03d6ee6cebf77ed08c"},
	{Index: " 12 ", Address: "one1t62r0ugj6g5jg5vjddenf663n7tk5p5y9t6rn0", BLSPublicKey: "55b364db6abed7f7409b32b9a6d5a19a9d8427ab0a3ed1caf0361fe5f65e2434b1930157ef9b8e7f2d93ad6c133f0b14"},
	{Index: " 13 ", Address: "one1awpctg90l33an0h3jgtvg2sz4zj25cw59c4tdq", BLSPublicKey: "5c8a4f0b6e9a59103df8749329eb3068208ad99e502727026cea82ea9737951031a215e8f67414f7de499d48740c9410"},
	{Index: " 14 ", Address: "one10yw6nlsrshku9evgwauj9uf9sjjvrtf8094gk3", BLSPublicKey: "5fbeeef2d5eb678b29d979c242f3cb0d317955c1c26daad88c12902d2016339463309b294b64f04e6c3b78f78a3c2094"},
	{Index: " 15 ", Address: "one16j8mpf9727g5x65dwaprdmhael3t57zy88phqh", BLSPublicKey: "62647f5af02309ba39a16d44cc241771697b69a36a8324d7302287fcbaae98080b1a8e923b6246e573ab659387c46b94"},
	{Index: " 16 ", Address: "one178y3878vs0l8mr98lfrnledewr37w4nr0h88c8", BLSPublicKey: "6ed4ef8c3986340f9418c00341904d6dc44505838a179cfc1d0be9e9b15bec9b44dc5d4d2403d277e0186f37cb2ac710"},
	{Index: " 17 ", Address: "one19scxzuhc2dx7knlujthxzq268muq8al3c50sw0", BLSPublicKey: "77a74330fed63063b9408b53004b9487d9e340ceb244794a11a97d8968fd691622bd3a9ffe30b073b2dd72efb15b7314"},
	{Index: " 18 ", Address: "one16a0zhskxrn6q0n3m9e0ce8krryvy3h30n38y98", BLSPublicKey: "79ca669cbbbef649a73be005440808e5ba81dee6013091b604e136ff4798f40fc719f6853f8ebbb39a3b251a13b6e504"},
	{Index: " 19 ", Address: "one1tuehnmc9d6euh9vlv5el3z85fmhw2vg6cprzl7", BLSPublicKey: "b1bba9d28613f04b244c4b7dd5d2cae961ecb2c6332ba8b9c7a12e5f13ed6008938251781b4f79bbaec3e6ceefc22794"},
	{Index: " 20 ", Address: "one1sveku8pkl85360xjn76nt5fpzwh8rqu9syjf6n", BLSPublicKey: "bb60f9b0c6a0ed588cc54a0416002226c61ca4c00c88aff42c76485dedf0760ad096ccf96dc889842bd8cd1b2e666384"},
	{Index: " 21 ", Address: "one1zkj9g3836jd47pa6vmpgcdg45ma97adss78d35", BLSPublicKey: "e0c650b6456b8b8adb43e848f5e612e51eb495d08d81e53f7aa906963930a73acbb4747ced96e0ca3b0d3cfbfc1fcd8c"},
}

// LocalHarmonyAccountsV1 are the accounts for the initial genesis nodes used for local test.
var LocalHarmonyAccountsV1 = []DeployAccount{
	{Index: " 0 ", Address: "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy", BLSPublicKey: "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204"},
	{Index: " 1 ", Address: "one1m6m0ll3q7ljdqgmth2t5j7dfe6stykucpj2nr5", BLSPublicKey: "40379eed79ed82bebfb4310894fd33b6a3f8413a78dc4d43b98d0adc9ef69f3285df05eaab9f2ce5f7227f8cb920e809"},
	{Index: " 2 ", Address: "one12fuf7x9rgtdgqg7vgq0962c556m3p7afsxgvll", BLSPublicKey: "02c8ff0b88f313717bc3a627d2f8bb172ba3ad3bb9ba3ecb8eed4b7c878653d3d4faf769876c528b73f343967f74a917"},
	{Index: " 3 ", Address: "one16qsd5ant9v94jrs89mruzx62h7ekcfxmduh2rx", BLSPublicKey: "ee2474f93cba9241562efc7475ac2721ab0899edf8f7f115a656c0c1f9ef8203add678064878d174bb478fa2e6630502"},
	{Index: " 4 ", Address: "one1pf75h0t4am90z8uv3y0dgunfqp4lj8wr3t5rsp", BLSPublicKey: "e751ec995defe4931273aaebcb2cd14bf37e629c554a57d3f334c37881a34a6188a93e76113c55ef3481da23b7d7ab09"},
	{Index: " 5 ", Address: "one1est2gxcvavmtnzc7mhd73gzadm3xxcv5zczdtw", BLSPublicKey: "776f3b8704f4e1092a302a60e84f81e476c212d6f458092b696df420ea19ff84a6179e8e23d090b9297dc041600bc100"},
	{Index: " 6 ", Address: "one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj", BLSPublicKey: "2d61379e44a772e5757e27ee2b3874254f56073e6bd226eb8b160371cc3c18b8c4977bd3dcb71fd57dc62bf0e143fd08"},
	{Index: " 7 ", Address: "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9", BLSPublicKey: "c4e4708b6cf2a2ceeb59981677e9821eebafc5cf483fb5364a28fa604cc0ce69beeed40f3f03815c9e196fdaec5f1097"},
	{Index: " 8 ", Address: "one1d2rngmem4x2c6zxsjjz29dlah0jzkr0k2n88wc", BLSPublicKey: "86dc2fdc2ceec18f6923b99fd86a68405c132e1005cf1df72dca75db0adfaeb53d201d66af37916d61f079f34f21fb96"},
	{Index: " 9 ", Address: "one1658znfwf40epvy7e46cqrmzyy54h4n0qa73nep", BLSPublicKey: "49d15743b36334399f9985feb0753430a2b287b2d68b84495bbb15381854cbf01bca9d1d9f4c9c8f18509b2bfa6bd40f"},
}

// LocalFnAccountsV1 are the accounts for the initial FN used for local test.
var LocalFnAccountsV1 = []DeployAccount{
	{Index: " 0 ", Address: "one1a50tun737ulcvwy0yvve0pvu5skq0kjargvhwe", BLSPublicKey: "00d600ed10af6905fe2ff63b3112d824ee6607adb82063c5cdee31781a94d1ebbc2bead7b307980de424ce641d14d594"},
	{Index: " 1 ", Address: "one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg", BLSPublicKey: "52e533007d9e08a2cce4301774a1b9f03e57c3e553baf782275efd4e3236dc785f3729b8d9404d61f4974766d3d35700"},
	{Index: " 2 ", Address: "one103q7qe5t2505lypvltkqtddaef5tzfxwsse4z7", BLSPublicKey: "6fda854ab94d722a5dbb3b0ffa9430ddda22e7489050702f8fd95c60f52db21be12b84960e87001055d5e744156b688c"},
	{Index: " 3 ", Address: "one129r9pj3sk0re76f7zs3qz92rggmdgjhtwge62k", BLSPublicKey: "8f744d023abe53030865c9e973e9ec146b16d5294a7f68455dc1829a9ae8109025a0b1170717f5709e2b3739a9dfcb08"},
	{Index: " 4 ", Address: "one1d2rngmem4x2c6zxsjjz29dlah0jzkr0k2n88wc", BLSPublicKey: "9201986fb2b34ceba38841335974d5618f79f6fe147e352533e8419d34407f0c265f2136a81b756a7a66e7ace72fb294"},
	{Index: " 5 ", Address: "one1658znfwf40epvy7e46cqrmzyy54h4n0qa73nep", BLSPublicKey: "9707ee8c8008dcf0955f9525c9d43aefdbda6fbae0afa55940ae8bce22149933dac46dbbb2d81207daebfcf3bb7e6c88"},

	//{Index: " 6 ", Address: "one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj", BLSPublicKey: "c33b121bd16f2cf502dab305380f02238907a44cacb1662366fb47058c8824cccec1486312bfe8d8ed7614acc2d5c018"},
	//{Index: " 7 ", Address: "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9", BLSPublicKey: "d3d32d50191d861980a4af5578d80a88bfb9d0c5f834d4e94285c632477a7b661339736a678c5172b149135b13693704"},
	//{Index: " 8 ", Address: "one1znqjvhlx8nzjvw2t0nkzzs6mcaxdy43xs5mspq", BLSPublicKey: "d7fd133b1633786ac516e93fdf503a4e055d9c9177a190f8c090fbba39f160310b19f49423bf771672b7437b92de5800"},
	//{Index: " 9 ", Address: "one1658znfwf40epvy7e46cqrmzyy54h4n0qa73nep", BLSPublicKey: "f321dd31c8ab3aa2efe09e194f07a4fa9195aa8aef1104d36234172ef6d1d6d365cc043196f40d763654c822f264c88c"},
	//{Index: " 10 ", Address: "one1z05g55zamqzfw9qs432n33gycdmyvs38xjemyl", BLSPublicKey: "4f8df320d3aea7cb1a4a55a175c721cee67f2f5cac6d6f5d66bc77d973e635e1af8c891db3a2856d3415a03ef0502f84"},
	//{Index: " 11 ", Address: "one1ljznytjyn269azvszjlcqvpcj6hjm822yrcp2e", BLSPublicKey: "346886d168a17a2bc971ba0850faa3c8943a0021f64806927a30c85b9ce9dd96a8941d7877646f40d4f9c630b2e82e04"},
}

// LocalHarmonyAccountsV2 are the accounts for the initial genesis nodes used for local test.
var LocalHarmonyAccountsV2 = []DeployAccount{
	{Index: " 0 ", Address: "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy", BLSPublicKey: "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204"},
	{Index: " 1 ", Address: "one1m6m0ll3q7ljdqgmth2t5j7dfe6stykucpj2nr5", BLSPublicKey: "40379eed79ed82bebfb4310894fd33b6a3f8413a78dc4d43b98d0adc9ef69f3285df05eaab9f2ce5f7227f8cb920e809"},
	{Index: " 2 ", Address: "one12fuf7x9rgtdgqg7vgq0962c556m3p7afsxgvll", BLSPublicKey: "02c8ff0b88f313717bc3a627d2f8bb172ba3ad3bb9ba3ecb8eed4b7c878653d3d4faf769876c528b73f343967f74a917"},
	{Index: " 3 ", Address: "one16qsd5ant9v94jrs89mruzx62h7ekcfxmduh2rx", BLSPublicKey: "ee2474f93cba9241562efc7475ac2721ab0899edf8f7f115a656c0c1f9ef8203add678064878d174bb478fa2e6630502"},
	{Index: " 4 ", Address: "one1pf75h0t4am90z8uv3y0dgunfqp4lj8wr3t5rsp", BLSPublicKey: "e751ec995defe4931273aaebcb2cd14bf37e629c554a57d3f334c37881a34a6188a93e76113c55ef3481da23b7d7ab09"},
	{Index: " 5 ", Address: "one1est2gxcvavmtnzc7mhd73gzadm3xxcv5zczdtw", BLSPublicKey: "776f3b8704f4e1092a302a60e84f81e476c212d6f458092b696df420ea19ff84a6179e8e23d090b9297dc041600bc100"},
	{Index: " 6 ", Address: "one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj", BLSPublicKey: "2d61379e44a772e5757e27ee2b3874254f56073e6bd226eb8b160371cc3c18b8c4977bd3dcb71fd57dc62bf0e143fd08"},
	{Index: " 7 ", Address: "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9", BLSPublicKey: "c4e4708b6cf2a2ceeb59981677e9821eebafc5cf483fb5364a28fa604cc0ce69beeed40f3f03815c9e196fdaec5f1097"},
	{Index: " 8 ", Address: "one1d2rngmem4x2c6zxsjjz29dlah0jzkr0k2n88wc", BLSPublicKey: "86dc2fdc2ceec18f6923b99fd86a68405c132e1005cf1df72dca75db0adfaeb53d201d66af37916d61f079f34f21fb96"},
	{Index: " 9 ", Address: "one1658znfwf40epvy7e46cqrmzyy54h4n0qa73nep", BLSPublicKey: "49d15743b36334399f9985feb0753430a2b287b2d68b84495bbb15381854cbf01bca9d1d9f4c9c8f18509b2bfa6bd40f"},
	{Index: " 10 ", Address: "one1z05g55zamqzfw9qs432n33gycdmyvs38xjemyl", BLSPublicKey: "95117937cd8c09acd2dfae847d74041a67834ea88662a7cbed1e170350bc329e53db151e5a0ef3e712e35287ae954818"},
	{Index: " 11 ", Address: "one1ljznytjyn269azvszjlcqvpcj6hjm822yrcp2e", BLSPublicKey: "68ae289d73332872ec8d04ac256ca0f5453c88ad392730c5741b6055bc3ec3d086ab03637713a29f459177aaa8340615"},
}

// LocalFnAccountsV2 are the accounts for the initial FN used for local test.
var LocalFnAccountsV2 = []DeployAccount{
	{Index: " 0 ", Address: "one1a50tun737ulcvwy0yvve0pvu5skq0kjargvhwe", BLSPublicKey: "52ecce5f64db21cbe374c9268188f5d2cdd5bec1a3112276a350349860e35fb81f8cfe447a311e0550d961cf25cb988d"},
	{Index: " 1 ", Address: "one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg", BLSPublicKey: "a547a9bf6fdde4f4934cde21473748861a3cc0fe8bbb5e57225a29f483b05b72531f002f8187675743d819c955a86100"},
	{Index: " 2 ", Address: "one103q7qe5t2505lypvltkqtddaef5tzfxwsse4z7", BLSPublicKey: "678ec9670899bf6af85b877058bea4fc1301a5a3a376987e826e3ca150b80e3eaadffedad0fedfa111576fa76ded980c"},
	{Index: " 3 ", Address: "one129r9pj3sk0re76f7zs3qz92rggmdgjhtwge62k", BLSPublicKey: "63f479f249c59f0486fda8caa2ffb247209489dae009dfde6144ff38c370230963d360dffd318cfb26c213320e89a512"},
	{Index: " 4 ", Address: "one1d2rngmem4x2c6zxsjjz29dlah0jzkr0k2n88wc", BLSPublicKey: "16513c487a6bb76f37219f3c2927a4f281f9dd3fd6ed2e3a64e500de6545cf391dd973cc228d24f9bd01efe94912e714"},
	{Index: " 5 ", Address: "one1658znfwf40epvy7e46cqrmzyy54h4n0qa73nep", BLSPublicKey: "576d3c48294e00d6be4a22b07b66a870ddee03052fe48a5abbd180222e5d5a1f8946a78d55b025de21635fd743bbad90"},
	{Index: " 6 ", Address: "one1ghkz3frhske7emk79p7v2afmj4a5t0kmjyt4s5", BLSPublicKey: "eca09c1808b729ca56f1b5a6a287c6e1c3ae09e29ccf7efa35453471fcab07d9f73cee249e2b91f5ee44eb9618be3904"},
	{Index: " 7 ", Address: "one1d7jfnr6yraxnrycgaemyktkmhmajhp8kl0yahv", BLSPublicKey: "f47238daef97d60deedbde5302d05dea5de67608f11f406576e363661f7dcbc4a1385948549b31a6c70f6fde8a391486"},
	{Index: " 8 ", Address: "one1r4zyyjqrulf935a479sgqlpa78kz7zlcg2jfen", BLSPublicKey: "fc4b9c535ee91f015efff3f32fbb9d32cdd9bfc8a837bb3eee89b8fff653c7af2050a4e147ebe5c7233dc2d5df06ee0a"},
	{Index: " 9 ", Address: "one1p7ht2d4kl8ve7a8jxw746yfnx4wnfxtp8jqxwe", BLSPublicKey: "ca86e551ee42adaaa6477322d7db869d3e203c00d7b86c82ebee629ad79cb6d57b8f3db28336778ec2180e56a8e07296"},
}
