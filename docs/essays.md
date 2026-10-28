# Сочинения

> Что исполнитель написал в 20 синтетических сочинениях на английском и что в них верно по плану (T82; R44). На них замер E4 меряет, как Claude оценивает сочинение по критериям автора — оценку по рубрике — и пишет отзыв (R41 spec; О-22). Как устроены вызов и отчёт, — [architecture.md](architecture.md), раздел 3.4, «Сочинения».
>
> Сочинения — файл `eval/essays.json` в формате `tutor-essays` v0 (R44). Это синтетическое содержание под Apache-2.0: сочинения написал исполнитель, а не ученики, на выдуманные темы и без имён (П-20). Файлов страниц нет: штрихи рисует код в памяти, из файла и зерна. Рубрика, счёт и сочинения ниже — тот же файл: тест `cmd/tutor-eval` собирает их из файла и сверяет с этим документом побайтно, так что сочинение правится в обоих местах.
>
> **Вычитка автора** подтверждает плановую истину как оценку взрослого: выполнен ли каждый критерий так, как сказано у сочинения. Где взрослый оценил бы иначе, — замечание к сочинению.

## Как читать

- **Рубрика** — одна на все сочинения, шесть критериев. Текст критерия — по-английски, как его дал бы модуль: язык объяснений модуля сочинений — английский (R41 spec). Модель получает критерии и тему блоком данных задачи (R02 spec).
- **Тема** — условие задачи, по-английски. Темы выдуманные и общие, не о жизни ученика, как рекомендует П-20.
- **Истина** — выполнен ли критерий, по плану. Пять критериев из шести программа считает сама и сверяет с планом при проверке файла, `tutor-eval check`. «По теме» держат только план и эта вычитка.
- **Без спорных случаев** — расхождение с истиной должно быть ошибкой модели, а не спором о правиле. Правила держит проверка файла, а то, чего программа не видит, — план и вычитка:
  - **предложения** — у выполненного критерия их 5–9, у невыполненного 3–4, длинные, через запятые. Предложение кончается точкой, вопросительным или восклицательным знаком, а после знака — пробел или конец текста;
  - **заглавные и точки** — не выполнено, если предложение начинается со строчной буквы, которая пишется не как своя заглавная: a b d e f g h i m n q r t y. У c, o, s, w и других заглавная отличается только ростом, и на почерке это спорно. Второй случай — последнее предложение без знака в конце. Пропущенной точки внутри текста нет: иначе спорно, сколько там предложений. Программа видит её, только если следующее слово — с заглавной, а перед строчной её держат план и вычитка;
  - **связующие слова** — выполнено, если разных связующих слов не меньше двух: because, but, so, then, first, next, finally, also, however, later, when. Не выполнено — ни одного, и даже and, or, after, before, while, until, if, although и since нет. Всё между этими случаями проверка отвергает. Программа считает слова, а не смысл: «so bright» и «the first customers» мыслей не связывают. Что у каждого сочинения, где критерий выполнен, и без таких слов связующих не меньше двух, держит вычитка;
  - **орфография** — не выполнено, если 2–3 частых слова написаны с ошибкой. Связующие слова с ошибкой не пишутся;
  - **по теме** — не выполнено, если всё сочинение — о другом предмете, и заголовок тоже;
  - **без имён** — заглавная буква внутри предложения стоит только у «I», в тексте и в теме. Так проверка держит сочинения без имён (П-20). Заголовок пишется с заглавных, и имя в нём находит только вычитка;
  - **знаки** — латинские буквы, апостроф внутри слова, пробел, запятая и знаки конца. В тексте — 40–80 слов, в заголовке — 1–6.
- **Доли** — каждый критерий не выполнен в 5 сочинениях из 20: в одном из разработки и в четырёх из теста. Всего не выполнено 30 критериев из 120. Все шесть выполняют 4 сочинения — одно в разработке и три в тесте, — а у остальных не выполнено от одного до трёх. Выполненных критериев больше: главная мера О-22 — критерий, который модель назвала невыполненным, хотя он выполнен.
- **Деление** — сочинения 1–5 — разработка, 6–20 — тест (T84).

## Как устроена страница

- **Лист** — 1000 × 1400 единиц, лист для рукописи формата `spec`: у задачи с рубрикой канва — `handwriting_sheet` с линейкой. Заголовок — первая строка, а текст под ним перенесён по словам, по 28–40 знаков в строке. Длину строки каждое сочинение берёт из зерна, так что и почерк у них разного размера.
- **Почерк** — синтетика `tutor`, латинские буквы — свои штрихи (R43). Между словами — промежуток в два пробела: в полпробела строки с вычислениями слова сливались.
- **Вырезка** — по правилу протокола eval при 32 px, как у всех замеров: только чернила. Масштаб задаёт медиана высот связок штрихов, а в тексте это рост строчной буквы: она и рисуется высотой D′, а заглавные — почти вдвое выше (В36). Где картинка при 32 px не укладывается в 1568 токенов, правило уменьшает D′. На 20 сочинениях D′ — 23–32 px, вырезка — 864–1216 × 671–1330 px, 936–1564 токена.
- **Посмотреть** вырезки — то, что увидит модель, — можно командой `go run ./cmd/tutor-eval essays -png synthetic/png/essays`. Каталог — в `synthetic/`, вне git.

## Рубрика

| ID | Текст критерия | Что значит | Истину сверяет |
|---|---|---|---|
| `title` | It has a title on its own first line. | у сочинения есть заголовок, на своей первой строке | программа: заголовок есть — от 1 до 6 слов, с заглавной буквы, без знака в конце |
| `sentences` | It has at least five sentences. | в сочинении не меньше пяти предложений | программа: предложения считаются по знакам конца |
| `capitals-and-stops` | Every sentence begins with a capital letter and ends with a full stop, a question mark or an exclamation mark. | каждое предложение начинается с заглавной буквы и кончается точкой, вопросительным или восклицательным знаком | программа |
| `linking-words` | It links its ideas with words such as because, but, so, then or first. | мысли связаны словами вроде because, but, so, then или first | программа, по закрытому списку связующих слов |
| `on-topic` | It keeps to the topic of the task. | сочинение держится темы задачи | только план и вычитка автора |
| `spelling` | Common words are spelled correctly. | частые слова написаны без ошибок | программа: слова с ошибкой перечислены в файле и стоят в тексте |

## Сколько каких сочинений

| Критерий | Не выполнен | Разработка | Тест |
|---|---|---|---|
| `title` | 5 | 1 | 4 |
| `sentences` | 5 | 1 | 4 |
| `capitals-and-stops` | 5 | 1 | 4 |
| `linking-words` | 5 | 1 | 4 |
| `on-topic` | 5 | 1 | 4 |
| `spelling` | 5 | 1 | 4 |
| **выполнены все** | 4 | 1 | 3 |

Разработка и тест — числа сочинений 1–5 и 6–20.

## Сочинения по одному

### 1. The Bread Robot

- **Часть:** разработка.
- **Тема:** Imagine a robot that bakes bread. Write a short essay about one day of its work.
- **Слов** — 69, **предложений** — 6.
- **Не выполнено:** ничего.

> The Bread Robot
>
> The robot wakes up at four in the morning because the ovens need time to get hot. First it mixes flour, water and salt in a big bowl. Then it shapes the dough into long loaves and round buns. The kitchen smells sweet and warm. When the sun comes up, the first customers knock on the door. The robot is tired, but it is happy to see them smile.

### 2. Under the Waves

- **Часть:** разработка.
- **Тема:** Imagine a town under the sea. Write a short essay about what a visitor sees there.
- **Слов** — 74, **предложений** — 3.
- **Не выполнено:**
  - `sentences` — 3 предложения.

> Under the Waves
>
> When I swim down to the town under the sea, I see houses made of shells, streets of soft white sand and gardens full of green weed that waves slowly in the water. The fish carry letters from house to house, and a crab sweeps the square every morning with a tiny broom. I would like to stay there for a long time, but I have to go back up because I need air.

### 3. My First Morning

- **Часть:** разработка.
- **Тема:** Imagine an island that nobody has found yet. Write a short essay about your first morning there.
- **Слов** — 68, **предложений** — 8.
- **Не выполнено:**
  - `spelling` — с ошибкой: realy вместо really, beautifull вместо beautiful.

> My First Morning
>
> I wake up on a quiet beach. The sand is warm and the sky is pink. First I walk to the trees to look for water. I find a small river with clear, cold water. Then I see a bird with long blue feathers. It is realy beautifull. I sit on a rock and wait for the sun. Nobody knows about this place, but I am not afraid.

### 4. без заголовка

- **Часть:** разработка.
- **Тема:** Imagine a tree that can talk. Write a short essay about what it tells you.
- **Слов** — 57, **предложений** — 6.
- **Не выполнено:**
  - `title` — заголовка нет;
  - `on-topic` — всё сочинение — о другом.

> The bicycle race starts at nine in the morning. Hundreds of riders wait at the line because the streets are closed for them. When the whistle blows, they rush forward. The crowd cheers loudly, but the wind is strong. Then the riders climb a steep hill near the river. The winner finishes in less than two hours.

### 5. The Flying Umbrella

- **Часть:** разработка.
- **Тема:** Imagine an umbrella that can fly. Write a short essay about where it goes.
- **Слов** — 52, **предложений** — 7.
- **Не выполнено:**
  - `capitals-and-stops` — предложение 5 начинается со строчной: «the umbrella …»;
  - `linking-words` — ни одного связующего слова, и даже and и or нет.

> The Flying Umbrella
>
> My red umbrella opens by itself on a windy day. It lifts me off the ground. We float over the roofs of the town. The people below look very small. the umbrella carries me to a hill far away. There it lands softly in the grass. It closes with a happy click.

### 6. The Lighthouse Cat

- **Часть:** тест.
- **Тема:** Imagine a cat that lives in a lighthouse. Write a short essay about one night of its life.
- **Слов** — 67, **предложений** — 6.
- **Не выполнено:** ничего.

> The Lighthouse Cat
>
> Every evening the cat climbs the long stairs of the lighthouse. At the top it sits next to the big lamp because the glass is warm. It watches the dark sea and the little boats. When a storm comes, the waves crash against the rocks. The cat is not scared, so it stays by the light all night. In the morning it sleeps on a soft blanket.

### 7. без заголовка

- **Часть:** тест.
- **Тема:** Imagine a library where the books whisper at night. Write a short essay about what you hear there.
- **Слов** — 68, **предложений** — 6.
- **Не выполнено:**
  - `title` — заголовка нет.

> The library is quiet in the day, but at night the books begin to whisper. I hide behind a tall shelf and listen. An old book of maps talks about mountains and deserts. A small book of poems sings a soft song. Then a book about the sea tells a story of a lost ship. I stay until the sun rises because every book has something to say.

### 8. The Shy Dragon

- **Часть:** тест.
- **Тема:** Imagine a dragon that is afraid of the dark. Write a short essay about how it finds its courage.
- **Слов** — 66, **предложений** — 9.
- **Не выполнено:**
  - `linking-words` — ни одного связующего слова, и даже and и or нет;
  - `spelling` — с ошибкой: freind вместо friend, beleive вместо believe.

> The Shy Dragon
>
> The little dragon lives in a deep cave. It does not like the dark. Every night it hides under its wings. One evening a small owl visits the cave. The owl is a kind freind. It shows the dragon the stars through a hole in the roof. The dragon can't beleive its eyes. It breathes a tiny flame. Its cave is full of warm light now.

### 9. A Balloon Ride

- **Часть:** тест.
- **Тема:** Imagine a snowman who wants to see the summer. Write a short essay about its wish.
- **Слов** — 70, **предложений** — 3.
- **Не выполнено:**
  - `sentences` — 3 предложения;
  - `on-topic` — всё сочинение — о другом.

> A Balloon Ride
>
> Early in the morning we climb into the basket of a huge yellow balloon, and the pilot lights a fire that roars above our heads. Slowly we rise over fields, farms and a river that shines like silver in the sun, so the cows below look like tiny toys. When we land in a soft meadow, everyone laughs because the basket tips over and we roll out onto the grass.

### 10. Above the Clouds

- **Часть:** тест.
- **Тема:** Imagine a city in the clouds. Write a short essay about a day you spend there.
- **Слов** — 64, **предложений** — 6.
- **Не выполнено:**
  - `capitals-and-stops` — последнее предложение — без знака в конце.

> Above the Clouds
>
> The city in the clouds has houses as light as feathers. People walk on bridges made of mist. In the morning I eat breakfast on a soft white hill. Then I ride a bird to the market because there are no cars. The fruit there tastes like rain and honey. When evening comes, the clouds turn orange and gold, so the whole city glows

### 11. The Walking Shoe

- **Часть:** тест.
- **Тема:** Imagine a shoe that can walk on its own. Write a short essay about where it goes.
- **Слов** — 66, **предложений** — 7.
- **Не выполнено:** ничего.

> The Walking Shoe
>
> One morning a brown shoe decides to go for a walk. First it hops down the steps of the house. Then it walks along the street and looks in the shop windows. A dog barks at it, but the shoe is brave. It visits the park because it wants to see the ducks. At sunset it comes home, tired and dusty. It sleeps well that night.

### 12. без заголовка

- **Часть:** тест.
- **Тема:** Imagine a garden on the roof of a tall building. Write a short essay about what grows there.
- **Слов** — 59, **предложений** — 6.
- **Не выполнено:**
  - `title` — заголовка нет;
  - `capitals-and-stops` — предложение 3 начинается со строчной: «bees come …»;
  - `spelling` — с ошибкой: wonderfull вместо wonderful, evry вместо every.

> High above the busy streets there is a garden on a roof. Tomatoes grow in big pots, and beans climb up long sticks. bees come from far away because the flowers are so bright. In the evening the wind is cool, but the plants are strong. The view of the city is wonderfull. I water the garden evry day.

### 13. The Fishing Boat

- **Часть:** тест.
- **Тема:** Imagine a clock that runs backwards. Write a short essay about a day when it changes everything.
- **Слов** — 61, **предложений** — 6.
- **Не выполнено:**
  - `on-topic` — всё сочинение — о другом.

> The Fishing Boat
>
> The little fishing boat leaves the port before dawn. The sailors pull the ropes, and the sail fills with wind. First they sail past the rocks where the seals sleep. Then they throw their nets into the cold water. The work is hard, but the sea is calm today. When the nets are full, the boat turns back to the port.

### 14. The Singing Whale

- **Часть:** тест.
- **Тема:** Imagine a whale that sings to ships. Write a short essay about its song.
- **Слов** — 62, **предложений** — 4.
- **Не выполнено:**
  - `sentences` — 4 предложения;
  - `linking-words` — ни одного связующего слова, и даже and и or нет.

> The Singing Whale
>
> Far out in the ocean lives a giant blue whale with a voice as deep as thunder. Every night it swims beside the big ships, singing a slow song about the moon, the stars, the warm southern seas. The sailors on the decks stop their work to listen, their eyes full of tears. The song stays in their hearts for many years.

### 15. без заголовка

- **Часть:** тест.
- **Тема:** Imagine a door in the side of a mountain. Write a short essay about what is behind it.
- **Слов** — 61, **предложений** — 6.
- **Не выполнено:**
  - `title` — заголовка нет;
  - `spelling` — с ошибкой: diffrent вместо different, peple вместо people.

> At the foot of the mountain there is a small green door. I knock three times, and it opens slowly. Behind it is a long tunnel with glowing stones on the walls. I follow the tunnel because I hear music. At the end I find a hall full of diffrent instruments. There are no peple, but the instruments play by themselves.

### 16. The Snail's Journey

- **Часть:** тест.
- **Тема:** Imagine a bus that can fly over the city. Write a short essay about a trip on it.
- **Слов** — 56, **предложений** — 7.
- **Не выполнено:**
  - `capitals-and-stops` — предложение 4 начинается со строчной: «its shell …»;
  - `linking-words` — ни одного связующего слова, и даже and и or нет;
  - `on-topic` — всё сочинение — о другом.

> The Snail's Journey
>
> A small snail starts a long journey across the garden. It moves very slowly over the wet stones. The sun is hot on its back. its shell feels heavy. A beetle passes by with a friendly nod. The snail rests under a big leaf. By evening it reaches the pond at the end of the garden.

### 17. The Fallen Star

- **Часть:** тест.
- **Тема:** Imagine a small star that falls into a field. Write a short essay about what happens next.
- **Слов** — 72, **предложений** — 7.
- **Не выполнено:** ничего.

> The Fallen Star
>
> Late one night a small star falls into a field of wheat. It glows softly, but it cannot fly back to the sky. A fox finds it first and sniffs it carefully. Then a sleepy owl lands beside them. The star is sad because it misses its friends. The fox and the owl carry it to the top of a hill. When the wind blows, the star rises back into the sky.

### 18. The Ice Bridge

- **Часть:** тест.
- **Тема:** Imagine a bridge made of ice. Write a short essay about crossing it.
- **Слов** — 64, **предложений** — 3.
- **Не выполнено:**
  - `sentences` — 3 предложения;
  - `spelling` — с ошибкой: slipery вместо slippery, beatiful вместо beautiful.

> The Ice Bridge
>
> The bridge over the frozen river is made of clear blue ice, and it shines like glass in the winter sun. I walk across it very carefully because the surface is slipery, but the view is so beatiful that I forget to be scared. When I reach the other side, I look back and see my footprints sparkling on the ice like tiny stars.

### 19. без заголовка

- **Часть:** тест.
- **Тема:** Imagine a painter who paints with the wind. Write a short essay about one of the pictures.
- **Слов** — 65, **предложений** — 3.
- **Не выполнено:**
  - `title` — заголовка нет;
  - `sentences` — 3 предложения;
  - `on-topic` — всё сочинение — о другом.

> In a quiet village there is a turtle that runs a tiny shop full of buttons, ribbons and old keys. Every customer has to wait a long time because the turtle walks very slowly from the shelves to the counter. Nobody minds the wait, so the shop is always busy, and children love to watch the turtle count the coins with its small green feet.

### 20. The Changing Map

- **Часть:** тест.
- **Тема:** Imagine a map that changes every night. Write a short essay about where it takes you.
- **Слов** — 59, **предложений** — 7.
- **Не выполнено:**
  - `capitals-and-stops` — предложение 3 начинается со строчной: «the path …», предложение 6 начинается со строчной: «each night …»;
  - `linking-words` — ни одного связующего слова, и даже and и or нет.

> The Changing Map
>
> My old map shows a different land every morning. Today it shows a desert with three red rocks. the path on the map leads to a hidden well. I walk for hours in the hot sand. The well is full of cold, sweet water. each night the map changes again. Tomorrow it might show a jungle full of parrots.
