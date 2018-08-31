(ns something.test)

(func main
    (var b Bar)
    (as b (Bar))

    (var x A<I>)
    (as x (A<I> -size 6))


    //(lisa Bar)
)

(global foo Str `234`)

(class Monkey : Bar Eater
    (m david
        (var s Str [rubber Bar])
        (as [zelda] 6.0)
        (lisa me)
        (var b (Bar))
    )

)

(interface Eater
    (m david)
    //(m david : I) // should be error because indistinguishable from previous overload
)



(class Bar
    (f -static rubber Str)
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
        (var i Str)
        (as i `jsidfj`)
    )
)