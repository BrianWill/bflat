(ns test)

(func main
    (var b Bar)
    (as b (Bar))

    (var x A<I>)
    (as x (A<I> -size 6))


    //(lisa Bar)
)


(class Monkey : Bar Eater

    (m david
        (var i 3)
        (as i 5)
        (as [zelda] 6.0)
        (lisa me)
    )
)

(interface Eater
    (m david))



(class Bar
    (f zelda F 35.0)

    (m lisa
        (var i B 3)
        (as i 5)
    )

    (constructor 
        (var i 3)
        (as i 5)
        //(lisa me)
    )

        (constructor : a Str
        (var i Str             a         )
        (as i `jsidfj`)
    )
)